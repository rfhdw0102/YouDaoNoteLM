package ppt

import "testing"

func TestValidateContentAcceptsCompactCanvasCSS(t *testing.T) {
	content := `<style>.ppt-slide{width:1920px;height:1080px;overflow:hidden}</style>
<section class="ppt-slide"><h1>一</h1><p>二</p></section>
<section class="ppt-slide"><h2>三</h2><p>四</p></section>
<section class="ppt-slide"><h2>五</h2><p>六</p></section>
<section class="ppt-slide"><h2>七</h2><p>八</p></section>`

	if !ValidateContent(content) {
		t.Fatal("compact but valid canvas CSS was rejected")
	}
}
