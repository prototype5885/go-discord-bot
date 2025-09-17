package main

type Response struct {
	Candidates    [1]Candidates
	UsageMetaData UsageMetadata
	Error         Error
}

type Candidates struct {
	Content       Content
	FinishReason  string
	Index         int
	SafetyRatings [4]SafetyRatings
}

// func newCandidates() *Candidates {
// 	return &Candidates{
// 		Index: -1,
// 	}
// }

type Content struct {
	Role  string
	Parts [1]Parts
}

type Contents struct {
	Role  string `json:"role"`
	Parts Parts  `json:"parts"`
}

type Conversation struct {
	Contents       []Contents        `json:"contents"`
	SafetySettings [4]SafetySettings `json:"safety_settings"`
}

func newConversation() *Conversation {
	return &Conversation{
		SafetySettings: *newSafetySettings(),
	}
}

type Parts struct {
	Text string `json:"text"`
}

type InlineData struct {
	Data     string `json:"data"`
	MimeType string `json:"mineType"`
}

type FileData struct {
	MimeType string
	FileUri  string
}

type PromptFeedback struct {
	BlockReason   string
	SafetyRatings [4]SafetyRatings
}

type SafetyRatings struct {
	Category    string
	Probability string
}

type UsageMetadata struct {
	PromptTokenCount     int
	CandidatesTokenCount int
	TotalTokenCount      int
}

// func newUsageMetaData() *UsageMetadata {
// 	return &UsageMetadata{
// 		PromptTokenCount:     -1,
// 		CandidatesTokenCount: -1,
// 		TotalTokenCount:      -1,
// 	}
// }

type Error struct {
	Code    int
	Message string
	Status  string
}

// func newError() *Error {
// 	return &Error{
// 		Code:    -1,
// 		Message: "Unknown error",
// 	}
// }

type SafetySettings struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

func newSafetySettings() *[4]SafetySettings {
	return &[4]SafetySettings{
		{
			Category:  "HARM_CATEGORY_SEXUALLY_EXPLICIT",
			Threshold: "BLOCK_NONE",
		},
		{
			Category:  "HARM_CATEGORY_HATE_SPEECH",
			Threshold: "BLOCK_NONE",
		},
		{
			Category:  "HARM_CATEGORY_HARASSMENT",
			Threshold: "BLOCK_NONE",
		},
		{
			Category:  "HARM_CATEGORY_DANGEROUS_CONTENT",
			Threshold: "BLOCK_NONE",
		},
	}
}

// var safetySettings = [4]SafetySettings{
// 	{
// 		Category:  "HARM_CATEGORY_SEXUALLY_EXPLICIT",
// 		Threshold: "BLOCK_NONE",
// 	},
// 	{
// 		Category:  "HARM_CATEGORY_HATE_SPEECH",
// 		Threshold: "BLOCK_NONE",
// 	},
// 	{
// 		Category:  "HARM_CATEGORY_HARASSMENT",
// 		Threshold: "BLOCK_NONE",
// 	},
// 	{
// 		Category:  "HARM_CATEGORY_DANGEROUS_CONTENT",
// 		Threshold: "BLOCK_NONE",
// 	},
// }

// type AttachmentContents struct {
// 	Role            string             `json:"role"`
// 	AttachmentParts [1]AttachmentParts `json:"parts"`
// }

// type AttachmentMessage struct {
// 	Contents       AttachmentContents `json:"contents"`
// 	SafetySettings [4]SafetySettings  `json:"safety_settings"`
// }

// type AttachmentParts struct {
// 	InlineData InlineData `json:"inlineData"`
// 	Text       string     `json:"text"`
// }
