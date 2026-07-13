# Token 吊销方案设计

> 解决「同一用户多页面/多设备登录时,一处登出不影响其他会话」的问题。

## 背景

### 当前架构

后端采用 **JWT + 单 token 黑名单**:

- 登录签发 access token(15m) + refresh token(24h),每个 token 携带唯一 JTI
- 登出时 `RevokeToken` 把传入的那一个 token 按 JTI 写入 Redis 黑名单(`token:blacklist:{jti}`)
- 中间件每次请求查黑名单,命中则拒绝

相关代码:
- `internal/service/token_blacklist_service.go` — 黑名单服务
- `internal/service/auth_service.go` — `Logout` 只吊销当前传入的 access/refresh token
- `internal/middleware/auth.go` — 中间件按 JTI 查黑名单

### 问题

JWT 是无状态的,服务端默认不知道「同一用户在两个页面登了两份」。单 token 拉黑不会波及其他会话:

- A 页面登出 → 只拉黑 A 页面那对 token
- B 页面用的是另一对 token(不同 JTI) → 不在黑名单 → 仍然有效

代码里其实已预留了用户级批量吊销的位:

```go
// internal/service/token_blacklist_service.go:14-16
tokenBlacklistKeyPrefix = "token:blacklist:"  // 单 token 黑名单(已实现)
userTokenKeyPrefix      = "token:user:"       // 用户 token 集合(用于批量吊销)← 常量定义了但从未引用
```

本方案就是把 `userTokenKeyPrefix` 这条预留路径补完。

---

## 方案对比

### 方案 A:用户级 token 集合

**实现原理**

1. 登录签发 token 时,把 JTI 写入 Redis 集合:`SADD token:user:{uid} {access_jti} {refresh_jti}`,集合 TTL 跟 refresh token 一致
2. 中间件不变,仍按 `token:blacklist:{jti}` 查单 token 黑名单
3. 登出时 `SMEMBERS token:user:{uid}` 取出该用户所有 JTI → 批量 `SET token:blacklist:{jti} 1 {ttl}` → `DEL token:user:{uid}`

**最大缺陷:集合与 token 生命周期会脱节**

- refresh 流程必须同步更新集合(SADD 新 JTI、SREM 旧 JTI),否则集合膨胀或漏吊销
- 并发登出 + refresh 存在竞态窗口
- 集合 TTL 必须和 refresh token TTL 严格对齐

### 方案 B:token 版本号

**实现原理**

1. `users` 表加 `token_version INT DEFAULT 0`
2. JWT claims 加 `v`,签发时写入当前 `user.token_version`
3. 中间件解析 token → 查 `user.token_version`(Redis 缓存)→ 不等就拒
4. 登出 `UPDATE users SET token_version = token_version + 1 WHERE id = ?`

**最大缺陷:中间件每次请求都要查版本号,缓存一致性是核心坑**

- 破坏 JWT 无状态特性,每次请求必查 user 表或 Redis
- 登出后必须立即失效缓存,否则旧 token 在窗口期内仍有效
- refresh 流程也要校验版本号

### 方案 C:全局会话表(有状态)

**实现原理**

1. 维护 session 表:`jti | user_id | expires_at | revoked`
2. 登录时 INSERT,中间件查存在性 + revoked 标志
3. 登出 `UPDATE session SET revoked=1 WHERE user_id=?`

**最大缺陷:完全抛弃 JWT 无状态优势,退化成 Session**

- 每次请求必查存储,跟传统 Cookie+Session 无本质区别
- session 表无限增长,需后台清理任务
- 多实例要共享存储,复杂度高

### 横向对比

| 维度 | A 集合 | B 版本号 | C 会话表 |
|---|---|---|---|
| 吊销粒度 | 用户级 | 用户级 | 用户级/单 token 可选 |
| 平时请求开销 | 0(只查黑名单,不变) | +1 次 Redis/DB 查询 | +1 次 Redis/DB 查询 |
| 登出开销 | N 次 SET(N=该用户 token 数) | 1 次 UPDATE | 1 次 UPDATE |
| 主要风险 | 集合与 token 生命周期脱节 | 缓存一致性窗口 | 性能 + 扩展性 |
| 代码改动量 | 中(登录/登出/refresh 三处) | 小(加字段 + 中间件 + 登出) | 大(整套 session 表 + 清理任务) |
| 适合场景 | 偶尔需要全设备登出 | 频繁需要全设备登出 | 强安全场景 |

---

## 选定方案:A. 用户级 token 集合

**理由**

- 平时请求 0 额外开销,中间件零改动,不影响主链路性能
- 登出是一次性操作,N 次 SET 可接受(用户 token 数通常 < 10)
- 不动数据库 schema,只动 Redis 与 auth_service/token_blacklist_service
- 代码里已预留 `userTokenKeyPrefix` 常量,顺势补完,改动范围明确
- 项目场景是「偶尔需要全设备登出」,不需要 B 那种频繁吊销

---

## 详细设计

### Redis 数据结构

| Key | 类型 | TTL | 用途 |
|---|---|---|---|
| `token:blacklist:{jti}` | String | token 剩余有效期 | 单 token 黑名单(已存在,不变) |
| `token:user:{uid}` | Set | 跟随 refresh token 有效期(24h) | 该用户所有在用 JTI 集合 |

集合元素示例:
```
token:user:2 = {
  "access_jti_a",  "refresh_jti_a",   // A 页面会话
  "access_jti_b",  "refresh_jti_b"    // B 页面会话
}
```

### 流程改动

#### 1. 登录(`auth_service.Login`)

签发 token 后,把两个 JTI 写入用户集合:

```go
// 伪代码
pipe := s.redis.Pipeline()
pipe.SAdd(ctx, userTokenKey(uid), accessJTI, refreshJTI)
pipe.Expire(ctx, userTokenKey(uid), refreshTTL)  // 24h
pipe.Exec(ctx)
```

注意:每次 SAdd 后都要重设 TTL(Redis Set 不支持元素级 TTL,只能整体过期)。

#### 2. 登出(`auth_service.Logout`) — 核心改动

```go
// 伪代码
jtis := s.redis.SMembers(ctx, userTokenKey(uid)).Val()  // 取出所有 JTI

pipe := s.redis.Pipeline()
for _, jti := range jtis {
    // 拉黑每个 token,TTL 取对应 token 的剩余有效期
    // 简化:统一用 access token 最大 TTL(15m),过期的 token 自然失效
    pipe.Set(ctx, blacklistKey(jti), "1", accessTTL)
}
pipe.Del(ctx, userTokenKey(uid))
pipe.Exec(ctx)
```

Logout 接口需要能拿到 `uid`(从当前请求的 access token 解析),不再只依赖前端传入的 token 字符串。

#### 3. Refresh(`auth_service.RefreshToken`)

refresh 会签发新 access token(新 JTI),必须同步更新集合:

```go
// 伪代码
pipe := s.redis.Pipeline()
pipe.SAdd(ctx, userTokenKey(uid), newAccessJTI)      // 加新 JTI
pipe.SRem(ctx, userTokenKey(uid), oldAccessJTI)      // 移除旧 JTI(可选,避免膨胀)
pipe.Expire(ctx, userTokenKey(uid), refreshTTL)       // 续期
pipe.Exec(ctx)
```

refresh token 本身不变(只换 access token),所以 refresh_jti 不动。

#### 4. 中间件(`middleware/auth.go`) — 不变

仍然只查 `token:blacklist:{jti}`,命中即拒。集合对中间件透明。

### 关键风险与对策

#### 风险 1:集合与 token 生命周期脱节

**场景**:refresh 没同步 SADD/SREM → 集合里残留已过期 JTI,或新 JTI 没进集合。

**对策**:
- refresh 流程必须用 Pipeline 原子执行 SADD + SREM + Expire
- 集合元素是 JTI(字符串),即使残留也只是一次无效的拉黑 SET,无安全影响
- 真正风险是「漏 SADD」导致登出漏吊销,所以 refresh 的 SADD 是必须项,SREM 是可选优化

#### 风险 2:并发登出 + refresh 竞态

**场景**:A 页面登出遍历集合的同时,B 页面 refresh 写入新 JTI,可能:
- 登出读到旧集合 → 拉黑旧 JTI,但新 JTI 没被拉黑 → B 页面继续有效

**对策**:
- 接受这个窗口(refresh 后的 token 算新会话,登出本就不该波及)
- 或者在登出 `DEL` 集合后,B 页面的 refresh 会 SADD 进空集合,下次登出能拉黑
- 严格语义需要 Lua 脚本原子化「SMEMBERS + 批量 SET + DEL」,但收益有限,不推荐过度设计

#### 风险 3:集合 TTL 与 refresh token TTL 不一致

**场景**:集合先过期 → 用户其实还登录着,但登出时集合空了拉黑不到。

**对策**:
- 每次登录/refresh 都 `Expire` 续期到 refresh token 的完整 TTL
- refresh token 续期时,集合也必须跟着续期

#### 风险 4:Redis 故障降级

**对策**:Redis 不可用时,黑名单查询失败 → 中间件默认放行(保持可用性优于安全性),与现有黑名单降级策略一致。

### 实现清单

改动点:

- [ ] `token_blacklist_interface.go`:新增 `RevokeUserTokens(ctx, uid) (int, error)` 接口方法
- [ ] `token_blacklist_service.go`:
  - [ ] 实现 `RevokeUserTokens`:SMEMBERS + 批量 SET + DEL
  - [ ] 新增 `AddUserToken(ctx, uid, jti, ttl)`:SADD + Expire
  - [ ] 新增 `RemoveUserToken(ctx, uid, jti)`:SREM
  - [ ] 实现 `userTokenKey(uid)` 辅助函数(用上已定义的 `userTokenKeyPrefix`)
- [ ] `auth_service.go`:
  - [ ] `Login`:签发后调用 `AddUserToken` 写入两个 JTI
  - [ ] `Logout`:从 access token 解析 uid,调用 `RevokeUserTokens(uid)`,再保留原有单 token 拉黑作为兜底
  - [ ] `RefreshToken`:换发新 access token 后 SADD 新 JTI + SREM 旧 JTI
- [ ] `auth/controller.go` `Logout`:确认能从 gin.Context 拿到当前用户 uid(中间件已注入)
- [ ] 中间件:无改动

### 测试要点

- 单页面登录登出:旧逻辑不退化
- 双页面:A 登出后 B 的 access/refresh token 都被拉黑
- refresh 后再登出:新 token 进集合,登出能拉黑到新 JTI
- 集合 TTL 续期:连续 refresh 后集合不过期
- Redis 故障降级:中间件不崩,黑名单查询失败时放行
