import type { Notebook, Source, Conversation, Note, QuizQuestion } from '../types';

const hourAgo = new Date(Date.now() - 3600000).toISOString();
const dayAgo = new Date(Date.now() - 86400000).toISOString();
const weekAgo = new Date(Date.now() - 604800000).toISOString();

// ============ Sources ============
const mockSources: Source[] = [
  {
    id: 'src-1',
    name: '深度学习基础.pdf',
    type: 'file',
    fileType: 'pdf',
    content: `# 深度学习基础

## 1. 神经网络概述

神经网络是一种模拟人脑神经元连接方式的计算模型。它由输入层、隐藏层和输出层组成。

### 1.1 感知机

感知机是最简单的神经网络，由 Frank Rosenblatt 在 1957 年提出。它接收多个输入信号，经过加权求和后通过激活函数输出。

\`\`\`python
import numpy as np

class Perceptron:
    def __init__(self, learning_rate=0.01, n_iters=1000):
        self.lr = learning_rate
        self.n_iters = n_iters
        self.activation_func = self._unit_step_func
        self.weights = None
        self.bias = None
\`\`\`

### 1.2 反向传播算法

反向传播（Backpropagation）是训练神经网络的核心算法。它通过链式法则计算损失函数对每个参数的梯度。

## 2. 卷积神经网络 (CNN)

CNN 是处理图像数据的核心架构，主要包含卷积层、池化层和全连接层。

### 2.1 卷积操作

卷积操作通过滑动窗口在输入特征图上提取局部特征。

### 2.2 经典架构

- **LeNet-5**: 最早的 CNN 之一，用于手写数字识别
- **AlexNet**: 2012 年 ImageNet 竞赛冠军，深度学习的里程碑
- **ResNet**: 引入残差连接，解决了深层网络的梯度消失问题

## 3. 循环神经网络 (RNN)

RNN 适用于序列数据的处理，如自然语言和时间序列。

### 3.1 LSTM

长短期记忆网络（LSTM）通过门控机制解决了标准 RNN 的长期依赖问题。

### 3.2 Transformer

Transformer 架构基于自注意力机制，是当前 NLP 领域的主流架构。`,
    size: 2457600,
    selected: true,
    createdAt: weekAgo,
    updatedAt: weekAgo,
  },
  {
    id: 'src-2',
    name: '机器学习综述.docx',
    type: 'file',
    fileType: 'docx',
    content: `# 机器学习综述

## 摘要

机器学习是人工智能的一个重要分支，它使计算机系统能够从数据中自动学习和改进。

## 1. 监督学习

监督学习是使用标注数据训练模型的方法。常见任务包括分类和回归。

### 主要算法
- 线性回归
- 逻辑回归
- 决策树
- 随机森林
- 支持向量机 (SVM)

## 2. 无监督学习

无监督学习处理没有标签的数据，目标是发现数据中的模式和结构。

### 主要方法
- K-Means 聚类
- 主成分分析 (PCA)
- 自编码器

## 3. 强化学习

强化学习通过与环境交互来学习最优策略，以最大化累积奖励。`,
    size: 1843200,
    selected: true,
    createdAt: dayAgo,
    updatedAt: dayAgo,
  },
  {
    id: 'src-3',
    name: 'Transformer 架构详解',
    type: 'url',
    content: `# Transformer 架构详解

## 注意力机制

Transformer 的核心是自注意力（Self-Attention）机制，它允许模型在处理序列中的每个位置时，关注序列中的所有其他位置。

### 缩放点积注意力

Attention(Q, K, V) = softmax(QK^T / √d_k) V

其中 Q、K、V 分别是查询、键和值矩阵。

### 多头注意力

多头注意力将注意力机制并行执行多次，每次使用不同的线性变换：

MultiHead(Q, K, V) = Concat(head_1, ..., head_h) W^O

## 位置编码

由于 Transformer 没有循环结构，需要额外的位置信息来表示序列中 token 的位置。

## 编码器-解码器结构

- **编码器**: N 个相同的层堆叠，每层包含多头自注意力和前馈网络
- **解码器**: 类似编码器，但增加了交叉注意力层`,
    url: 'https://example.com/transformer-explained',
    selected: false,
    createdAt: dayAgo,
    updatedAt: dayAgo,
  },
  {
    id: 'src-4',
    name: 'AI 研究会议录音.mp3',
    type: 'audio',
    fileType: 'mp3',
    content: `# AI 研究会议录音转写

## 会议主题：大语言模型的未来发展方向

**发言人 A**: 我认为大语言模型的发展有几个关键方向。首先是模型效率，我们需要更小但更强大的模型。

**发言人 B**: 同意。我特别关注模型压缩和量化技术。另外，多模态能力也是重要趋势。

**发言人 A**: 对，多模态模型能够理解和生成文本、图像、音频等多种类型的数据。

**发言人 C**: 我想补充一点，可解释性和安全性也是不可忽视的方向。随着模型能力的增强，我们需要更好的方法来理解和控制模型的行为。

**发言人 B**: 这让我想到了 RLHF（基于人类反馈的强化学习），它是目前提升模型安全性的主要方法之一。`,
    size: 52428800,
    selected: false,
    createdAt: weekAgo,
    updatedAt: weekAgo,
  },
];

// ============ Conversations ============
const mockConversations: Conversation[] = [
  {
    id: 'conv-1',
    title: '深度学习与机器学习的区别',
    notebookId: 'nb-1',
    messages: [
      {
        id: 'msg-1',
        role: 'user',
        content: '请帮我总结深度学习和传统机器学习的主要区别是什么？',
        timestamp: hourAgo,
      },
      {
        id: 'msg-2',
        role: 'assistant',
        content: `根据您的资料，深度学习和传统机器学习有以下主要区别：

**1. 特征工程**
- **传统机器学习**：需要人工设计和提取特征，特征的质量直接影响模型性能
- **深度学习**：能够自动学习特征表示，减少对人工特征工程的依赖

**2. 数据需求**
- **传统机器学习**：在小数据集上也能表现良好
- **深度学习**：通常需要大量标注数据才能达到最佳性能

**3. 计算资源**
- **传统机器学习**：计算需求相对较低
- **深度学习**：需要 GPU/TPU 等高性能计算资源

**4. 模型复杂度**
- **传统机器学习**：模型相对简单，如决策树、SVM
- **深度学习**：包含多层神经网络，参数量可达数十亿

**5. 可解释性**
- **传统机器学习**：通常更容易解释模型决策
- **深度学习**：常被称为"黑盒"，可解释性较差

> 引用来源：深度学习基础.pdf 第1章、机器学习综述.docx`,
        timestamp: hourAgo,
        citations: ['src-1', 'src-2'],
      },
      {
        id: 'msg-3',
        role: 'user',
        content: '那在实际应用中应该如何选择呢？',
        timestamp: new Date(Date.now() - 3000000).toISOString(),
      },
      {
        id: 'msg-4',
        role: 'assistant',
        content: `选择使用深度学习还是传统机器学习，主要考虑以下几个因素：

**选择传统机器学习的场景：**
- 数据量较小（< 1万条）
- 需要模型可解释性（如金融风控、医疗诊断）
- 计算资源有限
- 问题相对简单，特征明确

**选择深度学习的场景：**
- 数据量充足（> 10万条）
- 处理非结构化数据（图像、语音、文本）
- 问题复杂，特征难以人工定义
- 有足够的计算资源

**实践建议：**
1. 先尝试简单的模型作为基线
2. 根据性能需求决定是否升级到深度学习
3. 考虑使用集成学习方法结合两者优势`,
        timestamp: new Date(Date.now() - 2700000).toISOString(),
        citations: ['src-1', 'src-2'],
      },
    ],
    createdAt: hourAgo,
    updatedAt: new Date(Date.now() - 2700000).toISOString(),
  },
  {
    id: 'conv-2',
    title: 'Transformer 模型的工作原理',
    notebookId: 'nb-1',
    messages: [
      {
        id: 'msg-5',
        role: 'user',
        content: '请详细解释 Transformer 模型的工作原理',
        timestamp: dayAgo,
      },
      {
        id: 'msg-6',
        role: 'assistant',
        content: `Transformer 是当前 NLP 领域最重要的模型架构，让我为您详细解析：

## 核心机制：自注意力

自注意力机制让模型能够关注输入序列中的所有位置：

\`\`\`
Attention(Q, K, V) = softmax(QK^T / √d_k) V
\`\`\`

## 主要组件

1. **多头注意力**：并行执行多次注意力计算，捕捉不同维度的信息
2. **位置编码**：为序列中的每个位置添加位置信息
3. **前馈网络**：两层全连接网络，进行非线性变换
4. **残差连接**：缓解梯度消失问题

## 优势

- **并行计算**：相比 RNN，Transformer 可以并行处理整个序列
- **长距离依赖**：通过注意力机制直接建立远距离的关联
- **可扩展性**：模型规模可以方便地扩展

> 引用来源：Transformer 架构详解`,
        timestamp: dayAgo,
        citations: ['src-3'],
      },
    ],
    createdAt: dayAgo,
    updatedAt: dayAgo,
  },
];

// ============ Notes ============
const mockNotes: Note[] = [
  {
    id: 'note-1',
    title: '机器学习算法速查表',
    type: 'note',
    content: `# 机器学习算法速查表

## 监督学习

| 算法 | 类型 | 适用场景 | 优点 | 缺点 |
|------|------|----------|------|------|
| 线性回归 | 回归 | 连续值预测 | 简单、可解释 | 无法处理非线性 |
| 逻辑回归 | 分类 | 二分类问题 | 计算效率高 | 特征需线性可分 |
| 决策树 | 分类/回归 | 通用 | 可解释性强 | 容易过拟合 |
| 随机森林 | 分类/回归 | 通用 | 抗过拟合 | 计算量大 |
| SVM | 分类 | 小样本高维 | 泛化能力强 | 大数据集慢 |

## 无监督学习

| 算法 | 类型 | 适用场景 |
|------|------|----------|
| K-Means | 聚类 | 客户分群 |
| PCA | 降维 | 数据可视化 |
| 自编码器 | 降维/生成 | 特征学习 |

## 深度学习

| 架构 | 适用场景 |
|------|----------|
| CNN | 图像处理 |
| RNN/LSTM | 序列数据 |
| Transformer | NLP/多模态 |`,
    isSource: false,
    notebookId: 'nb-1',
    createdAt: dayAgo,
    updatedAt: dayAgo,
  },
  {
    id: 'note-2',
    title: '思维导图：深度学习架构',
    type: 'mindmap',
    content: `# 深度学习架构

## 基础架构
- 全连接网络 (FCN)
  - 输入层
  - 隐藏层
  - 输出层

## 卷积网络 (CNN)
- 卷积层
  - 卷积核
  - 步长
  - 填充
- 池化层
  - 最大池化
  - 平均池化
- 经典模型
  - LeNet
  - AlexNet
  - VGG
  - ResNet
  - EfficientNet

## 序列模型
- RNN
  - 梯度消失问题
- LSTM
  - 遗忘门
  - 输入门
  - 输出门
- GRU
  - 更新门
  - 重置门

## 注意力模型
- Transformer
  - 自注意力
  - 多头注意力
  - 位置编码
- BERT
  - 预训练
  - 微调
- GPT
  - 自回归生成
  - 指令跟随`,
    isSource: false,
    notebookId: 'nb-1',
    createdAt: hourAgo,
    updatedAt: hourAgo,
  },
  {
    id: 'note-3',
    title: '深度学习基础测验',
    type: 'quiz',
    content: JSON.stringify({
      questions: [
        {
          id: 'q1',
          question: '以下哪个不是深度学习中常用的激活函数？',
          options: ['ReLU', 'Sigmoid', 'Tanh', '线性函数'],
          correctIndex: 3,
          explanation: '线性函数不是常用的激活函数。深度学习中常用的激活函数包括 ReLU、Sigmoid、Tanh、Leaky ReLU 等。线性函数作为激活函数无法引入非线性，导致多层网络等价于单层。',
        },
        {
          id: 'q2',
          question: 'CNN 中卷积操作的主要目的是什么？',
          options: ['减少参数数量', '提取局部特征', '增加非线性', '归一化数据'],
          correctIndex: 1,
          explanation: '卷积操作的主要目的是提取局部特征。通过滑动窗口在输入上提取局部模式，卷积核可以学习到边缘、纹理等视觉特征。',
        },
        {
          id: 'q3',
          question: 'Transformer 模型中使用的核心机制是什么？',
          options: ['循环连接', '卷积操作', '自注意力机制', '池化操作'],
          correctIndex: 2,
          explanation: 'Transformer 的核心机制是自注意力（Self-Attention），它允许模型在处理每个位置时关注输入序列的所有位置，从而捕捉长距离依赖关系。',
        },
        {
          id: 'q4',
          question: 'ResNet 解决了什么问题？',
          options: ['过拟合', '梯度消失', '计算效率', '数据不足'],
          correctIndex: 1,
          explanation: 'ResNet 通过引入残差连接（skip connections）解决了深层网络中的梯度消失问题，使得训练非常深的网络成为可能。',
        },
        {
          id: 'q5',
          question: 'LSTM 相比标准 RNN 的主要改进是什么？',
          options: ['更多层数', '门控机制', '注意力机制', '卷积操作'],
          correctIndex: 1,
          explanation: 'LSTM 通过引入门控机制（遗忘门、输入门、输出门）来控制信息的流动，从而解决了标准 RNN 的长期依赖问题。',
        },
      ],
    } as any),
    isSource: false,
    notebookId: 'nb-1',
    createdAt: new Date(Date.now() - 1800000).toISOString(),
    updatedAt: new Date(Date.now() - 1800000).toISOString(),
  },
  {
    id: 'note-4',
    title: '深度学习概述 PPT',
    type: 'ppt',
    content: `<html><head><style>
body { font-family: 'Noto Sans SC', sans-serif; background: #0f1117; color: #E8E6E3; margin: 0; }
.slide { width: 100%; height: 100vh; display: flex; flex-direction: column; justify-content: center; align-items: center; padding: 60px; box-sizing: border-box; }
h1 { font-size: 48px; background: linear-gradient(135deg, #6C63FF, #4ECDC4); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
h2 { font-size: 36px; color: #6C63FF; }
p, li { font-size: 20px; line-height: 1.8; }
ul { text-align: left; }
.slide { border-bottom: 1px solid #1E2130; }
</style></head><body>
<div class="slide"><h1>深度学习概述</h1><p>从基础到前沿</p></div>
<div class="slide"><h2>什么是深度学习？</h2><ul><li>机器学习的一个子领域</li><li>使用多层神经网络</li><li>自动学习特征表示</li></ul></div>
<div class="slide"><h2>核心架构</h2><ul><li>CNN - 卷积神经网络（图像）</li><li>RNN/LSTM - 循环神经网络（序列）</li><li>Transformer - 自注意力模型（通用）</li></ul></div>
<div class="slide"><h2>应用场景</h2><ul><li>计算机视觉</li><li>自然语言处理</li><li>语音识别</li><li>推荐系统</li></ul></div>
</body></html>`,
    isSource: false,
    notebookId: 'nb-1',
    createdAt: new Date(Date.now() - 900000).toISOString(),
    updatedAt: new Date(Date.now() - 900000).toISOString(),
  },
];

// ============ Notebooks ============
export const mockNotebooks: Notebook[] = [
  {
    id: 'nb-1',
    name: '深度学习研究',
    description: '关于深度学习和机器学习的学习资料整理',
    sources: mockSources,
    conversations: mockConversations,
    notes: mockNotes,
    createdAt: weekAgo,
    updatedAt: hourAgo,
  },
  {
    id: 'nb-2',
    name: '产品设计笔记',
    description: '产品设计方法论和案例分析',
    sources: [],
    conversations: [],
    notes: [],
    createdAt: weekAgo,
    updatedAt: dayAgo,
  },
  {
    id: 'nb-3',
    name: '论文阅读',
    description: '学术论文阅读笔记和总结',
    sources: [],
    conversations: [],
    notes: [],
    createdAt: dayAgo,
    updatedAt: dayAgo,
  },
];

// ============ Quiz Questions (exported for QuizCard) ============
export const mockQuizQuestions: QuizQuestion[] = [
  {
    id: 'q1',
    question: '以下哪个不是深度学习中常用的激活函数？',
    options: ['ReLU', 'Sigmoid', 'Tanh', '线性函数'],
    correctIndex: 3,
    explanation: '线性函数不是常用的激活函数。深度学习中常用的激活函数包括 ReLU、Sigmoid、Tanh、Leaky ReLU 等。线性函数作为激活函数无法引入非线性，导致多层网络等价于单层。',
  },
  {
    id: 'q2',
    question: 'CNN 中卷积操作的主要目的是什么？',
    options: ['减少参数数量', '提取局部特征', '增加非线性', '归一化数据'],
    correctIndex: 1,
    explanation: '卷积操作的主要目的是提取局部特征。通过滑动窗口在输入上提取局部模式，卷积核可以学习到边缘、纹理等视觉特征。',
  },
  {
    id: 'q3',
    question: 'Transformer 模型中使用的核心机制是什么？',
    options: ['循环连接', '卷积操作', '自注意力机制', '池化操作'],
    correctIndex: 2,
    explanation: 'Transformer 的核心机制是自注意力（Self-Attention），它允许模型在处理每个位置时关注输入序列的所有位置，从而捕捉长距离依赖关系。',
  },
  {
    id: 'q4',
    question: 'ResNet 解决了什么问题？',
    options: ['过拟合', '梯度消失', '计算效率', '数据不足'],
    correctIndex: 1,
    explanation: 'ResNet 通过引入残差连接（skip connections）解决了深层网络中的梯度消失问题，使得训练非常深的网络成为可能。',
  },
  {
    id: 'q5',
    question: 'LSTM 相比标准 RNN 的主要改进是什么？',
    options: ['更多层数', '门控机制', '注意力机制', '卷积操作'],
    correctIndex: 1,
    explanation: 'LSTM 通过引入门控机制（遗忘门、输入门、输出门）来控制信息的流动，从而解决了标准 RNN 的长期依赖问题。',
  },
];
