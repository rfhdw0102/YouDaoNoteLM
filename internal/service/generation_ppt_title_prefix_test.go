package service

import "testing"

func TestStripPPTBulletSlideTitlePrefix(t *testing.T) {
	cases := []struct {
		name   string
		bullet string
		title  string
		want   string
	}{
		{
			name:   "chinese colon prefix",
			bullet: "卡尔文循环：场所：叶绿体基质",
			title:  "卡尔文循环",
			want:   "场所：叶绿体基质",
		},
		{
			name:   "ascii colon prefix",
			bullet: "卡尔文循环:CO₂固定:与RuBP结合",
			title:  "卡尔文循环",
			want:   "CO₂固定:与RuBP结合",
		},
		{
			name:   "space separator prefix",
			bullet: "光合作用 光反应阶段在类囊体膜上进行",
			title:  "光合作用",
			want:   "光反应阶段在类囊体膜上进行",
		},
		{
			name:   "repeated prefix stripped iteratively",
			bullet: "卡尔文循环：卡尔文循环：场所：叶绿体基质",
			title:  "卡尔文循环",
			want:   "场所：叶绿体基质",
		},
		{
			name:   "bracketed chapter title matches core",
			bullet: "卡尔文循环：场所：叶绿体基质",
			title:  "卡尔文循环（一）",
			want:   "场所：叶绿体基质",
		},
		{
			name:   "no separator after title keeps bullet intact",
			bullet: "封面页内容介绍",
			title:  "封面",
			want:   "封面页内容介绍",
		},
		{
			name:   "title too short no strip",
			bullet: "A：something",
			title:  "A",
			want:   "A：something",
		},
		{
			name:   "bullet does not start with title",
			bullet: "暗反应不依赖光",
			title:  "光反应",
			want:   "暗反应不依赖光",
		},
		{
			name:   "strip would blank bullet keeps original",
			bullet: "卡尔文循环：",
			title:  "卡尔文循环",
			want:   "卡尔文循环：",
		},
		{
			name:   "case insensitive english",
			bullet: "Photosynthesis: light reactions",
			title:  "photosynthesis",
			want:   "light reactions",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripPPTBulletSlideTitlePrefix(c.bullet, c.title)
			if got != c.want {
				t.Errorf("stripPPTBulletSlideTitlePrefix(%q, %q) = %q, want %q", c.bullet, c.title, got, c.want)
			}
		})
	}
}

func TestStripPPTHTMLRepeatedTitlePrefix(t *testing.T) {
	html := `<section class="ppt-slide" data-ppt-slide="true"><h2>卡尔文循环</h2><ul><li>卡尔文循环：场所：叶绿体基质</li><li>卡尔文循环：CO₂固定：与RuBP结合</li></ul></section>`
	want := `<section class="ppt-slide" data-ppt-slide="true"><h2>卡尔文循环</h2><ul><li>场所：叶绿体基质</li><li>CO₂固定：与RuBP结合</li></ul></section>`
	got := stripPPTHTMLRepeatedTitlePrefix(html)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStripPPTHTMLRepeatedTitlePrefixLeavesHeadingIntact(t *testing.T) {
	// The heading text itself must not be stripped even though it equals the
	// title used for bullet stripping.
	html := `<section><h2>光合作用</h2><p>光合作用：光反应</p></section>`
	want := `<section><h2>光合作用</h2><p>光反应</p></section>`
	got := stripPPTHTMLRepeatedTitlePrefix(html)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStripPPTHTMLRepeatedTitlePrefixSkipsStyle(t *testing.T) {
	// CSS rules like ".foo:bar" must not be touched.
	html := `<section><h2>foo</h2><style>.foo:bar{color:red}</style><p>foo: value</p></section>`
	want := `<section><h2>foo</h2><style>.foo:bar{color:red}</style><p>value</p></section>`
	got := stripPPTHTMLRepeatedTitlePrefix(html)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStripPPTHTMLRepeatedTitlePrefixSubheading(t *testing.T) {
	// The repeated prefix is a sub-heading (h3) under the section heading (h2),
	// not the main heading. The pass must collect h3 as a candidate too.
	html := `<section><h2>卡尔文循环</h2><h3>暗反应</h3><p>暗反应：场所：叶绿体基质</p><p>暗反应：前置条件：光反应提供ATP和NADPH</p></section>`
	want := `<section><h2>卡尔文循环</h2><h3>暗反应</h3><p>场所：叶绿体基质</p><p>前置条件：光反应提供ATP和NADPH</p></section>`
	got := stripPPTHTMLRepeatedTitlePrefix(html)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStripPPTHTMLRepeatedTitlePrefixCardTitle(t *testing.T) {
	// Card layout: the card-title is the repeated prefix inside the card body.
	html := `<section><h2>光合作用</h2><div class="card-grid"><div class="content-card"><div class="card-title">暗反应</div><div class="card-body">暗反应：场所：叶绿体基质</div></div></div></section>`
	want := `<section><h2>光合作用</h2><div class="card-grid"><div class="content-card"><div class="card-title">暗反应</div><div class="card-body">场所：叶绿体基质</div></div></div></section>`
	got := stripPPTHTMLRepeatedTitlePrefix(html)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStripPPTBulletTitlePrefixesStacked(t *testing.T) {
	// Two different title prefixes stacked on one node: peel both.
	got := stripPPTBulletTitlePrefixes("卡尔文循环：暗反应：场所：叶绿体基质", []string{"卡尔文循环", "暗反应"})
	want := "场所：叶绿体基质"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripPPTBulletTitlePrefixesIgnoresShortCandidates(t *testing.T) {
	// A 1-rune candidate must be ignored so it can't over-trim.
	got := stripPPTBulletTitlePrefixes("光反应：阶段", []string{"光", "光反应"})
	want := "阶段"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

