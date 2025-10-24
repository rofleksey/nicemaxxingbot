package openai

type AnalyzeResult struct {
	Toxic   bool   `json:"toxic"`
	Phrase  string `json:"phrase"`
	TurnOff bool   `json:"off"`
	TurnOn  bool   `json:"on"`
}
