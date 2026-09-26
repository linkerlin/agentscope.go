package askuser

import (
	"context"
	"strings"
	"testing"
)

func validQuestions() []Question {
	return []Question{
		{
			Question: "Which database should we use?",
			Header:   "Database",
			Options: []Option{
				{Label: "Postgres (Recommended)", Description: "Reliable relational database"},
				{Label: "SQLite", Description: "Zero-ops embedded database"},
			},
		},
	}
}

func TestAskUserTool_Flags(t *testing.T) {
	tl := New()
	if !tl.IsExternalTool() {
		t.Fatal("ask_user must be an external tool")
	}
	if !tl.IsReadOnly() {
		t.Fatal("ask_user must be read-only")
	}
	if !tl.IsAutoApproved() {
		t.Fatal("ask_user must be auto-approved")
	}
	if _, err := tl.Execute(context.Background(), nil); err == nil {
		t.Fatal("direct Execute must fail for an external tool")
	}
}

func TestAskUserTool_SpecRequiresQuestions(t *testing.T) {
	spec := New().Spec()
	required, _ := spec.Parameters["required"].([]string)
	if len(required) != 1 || required[0] != "questions" {
		t.Fatalf("expected questions required, got %v", required)
	}
	props, _ := spec.Parameters["properties"].(map[string]any)
	q, _ := props["questions"].(map[string]any)
	if q["minItems"] != 1 || q["maxItems"] != 4 {
		t.Fatalf("unexpected questions bounds: %+v", q)
	}
}

func TestValidateQuestions(t *testing.T) {
	if err := ValidateQuestions(validQuestions()); err != nil {
		t.Fatalf("valid questions rejected: %v", err)
	}

	multiSelectOK := validQuestions()
	multiSelectOK[0].MultiSelect = true
	if err := ValidateQuestions(multiSelectOK); err != nil {
		t.Fatalf("multi-select without previews rejected: %v", err)
	}

	tooLongHeader := validQuestions()
	tooLongHeader[0].Header = "this header is far too long"
	if err := ValidateQuestions(tooLongHeader); err == nil {
		t.Fatal("expected header length error")
	}

	unicodeHeader := validQuestions()
	unicodeHeader[0].Header = "数据库选型" // 5 runes
	if err := ValidateQuestions(unicodeHeader); err != nil {
		t.Fatalf("unicode header within limit rejected: %v", err)
	}

	tooFewOptions := validQuestions()
	tooFewOptions[0].Options = tooFewOptions[0].Options[:1]
	if err := ValidateQuestions(tooFewOptions); err == nil {
		t.Fatal("expected option count error")
	}

	otherOption := validQuestions()
	otherOption[0].Options[1].Label = "Other"
	if err := ValidateQuestions(otherOption); err == nil {
		t.Fatal("expected Other option rejection")
	}

	dupLabel := validQuestions()
	dupLabel[0].Options[1].Label = dupLabel[0].Options[0].Label
	if err := ValidateQuestions(dupLabel); err == nil {
		t.Fatal("expected duplicate label error")
	}

	dupQuestion := validQuestions()
	dupQuestion = append(dupQuestion, dupQuestion[0])
	if err := ValidateQuestions(dupQuestion); err == nil {
		t.Fatal("expected duplicate question error")
	}

	previewMulti := validQuestions()
	previewMulti[0].MultiSelect = true
	previewMulti[0].Options[0].Preview = "some code"
	if err := ValidateQuestions(previewMulti); err == nil {
		t.Fatal("expected multi-select preview rejection")
	}

	empty := []Question{}
	if err := ValidateQuestions(empty); err == nil {
		t.Fatal("expected empty questions rejection")
	}

	tooMany := append(validQuestions(), validQuestions()...)
	tooMany = append(tooMany, validQuestions()...)
	tooMany = append(tooMany, validQuestions()...)
	if err := ValidateQuestions(tooMany); err == nil {
		t.Fatal("expected question count error")
	}
}

func TestParseAndFormatMetadata(t *testing.T) {
	payload := `{"answers":[{"question":"Which database?","selected":["SQLite"],"other":"or MySQL"}]}`
	meta, err := ParseMetadata(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Answers) != 1 || meta.Answers[0].Selected[0] != "SQLite" || meta.Answers[0].Other != "or MySQL" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	text := FormatAnswersText(meta)
	if !strings.Contains(text, "SQLite") || !strings.Contains(text, "or MySQL") {
		t.Fatalf("unexpected formatted answers: %q", text)
	}

	if _, err := ParseMetadata("not json"); err == nil {
		t.Fatal("expected parse error for invalid payload")
	}
	if got := FormatAnswersText(nil); !strings.Contains(got, "did not answer") {
		t.Fatalf("unexpected empty answers text: %q", got)
	}
	if got := FormatAnswersText(&Metadata{Answers: []Answer{{Question: "q"}}}); !strings.Contains(got, "(no answer)") {
		t.Fatalf("unexpected no-answer text: %q", got)
	}
}

func TestMetadataSchema(t *testing.T) {
	schema := MetadataSchema()
	if schema["type"] != "object" {
		t.Fatalf("unexpected schema: %+v", schema)
	}
}
