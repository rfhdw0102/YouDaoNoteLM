package ppt

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

func fixPPTXPackage(data []byte, slideCount int) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(reader.File))
	for _, file := range reader.File {
		existing[file.Name] = true
	}

	needsPresentationXML := true
	missing := make([]string, 0, slideCount)
	for i := 1; i <= slideCount; i++ {
		name := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i)
		if !existing[name] {
			missing = append(missing, name)
		}
	}

	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, file := range reader.File {
		if file.Name == "ppt/presentation.xml" {
			if err := writePPTXZipEntry(writer, file.Name, []byte(pptxPresentationXML(slideCount))); err != nil {
				_ = writer.Close()
				return nil, err
			}
			needsPresentationXML = false
			continue
		}
		if file.Name == "ppt/slideMasters/slideMaster1.xml" {
			if err := writePPTXZipEntry(writer, file.Name, []byte(pptxSlideMasterXML())); err != nil {
				_ = writer.Close()
				return nil, err
			}
			continue
		}
		if err := copyPPTXZipEntry(writer, file); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if needsPresentationXML {
		if err := writePPTXZipEntry(writer, "ppt/presentation.xml", []byte(pptxPresentationXML(slideCount))); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	for _, name := range missing {
		if err := writePPTXZipEntry(writer, name, []byte(pptxSlideRelationshipXML())); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writePPTXZipEntry(writer *zip.Writer, name string, data []byte) error {
	entry, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}

func copyPPTXZipEntry(writer *zip.Writer, file *zip.File) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	header := file.FileHeader
	entry, err := writer.CreateHeader(&header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, reader)
	return err
}

func pptxPresentationXML(slideCount int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst>
    <p:sldMasterId id="2147483648" r:id="rId2"/>
  </p:sldMasterIdLst>
  <p:sldIdLst>
`)
	for i := 1; i <= slideCount; i++ {
		b.WriteString(fmt.Sprintf(`    <p:sldId id="%d" r:id="rId%d"/>
`, 255+i, i+2))
	}
	b.WriteString(`  </p:sldIdLst>
  <p:sldSz cx="12192000" cy="6858000" type="screen16x9"/>
  <p:notesSz cx="6858000" cy="9144000"/>
  <p:defaultTextStyle/>
</p:presentation>`)
	return b.String()
}

func pptxSlideMasterXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
        <p:cNvGrpSpPr/>
        <p:nvPr/>
      </p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
  <p:sldLayoutIdLst>
    <p:sldLayoutId id="2147483649" r:id="rId1"/>
  </p:sldLayoutIdLst>
  <p:txStyles>
    <p:titleStyle/>
    <p:bodyStyle/>
    <p:otherStyle/>
  </p:txStyles>
</p:sldMaster>`
}

func pptxSlideRelationshipXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`
}
