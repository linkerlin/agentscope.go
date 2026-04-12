package output

import "testing"

func TestParseJSONFromAssistant(t *testing.T) {
	var out struct {
		X int `json:"x"`
	}
	err := ParseJSONFromAssistant("here {\"x\": 2}", &out)
	if err != nil || out.X != 2 {
		t.Fatal(err, out.X)
	}
}
