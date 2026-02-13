package importer

// OfficialTestCase rappresenta un test case dal formato JSON ufficiale.
type OfficialTestCase struct {
	// Expression da valutare
	Expr string `json:"expr"`
	
	// Input data inline (opzionale)
	Data interface{} `json:"data,omitempty"`
	
	// Nome dataset da caricare (opzionale)
	Dataset *string `json:"dataset,omitempty"`
	
	// Variable bindings
	Bindings map[string]interface{} `json:"bindings,omitempty"`
	
	// Timelimit in millisecondi
	Timelimit *int `json:"timelimit,omitempty"`
	
	// Max evaluation depth
	Depth *int `json:"depth,omitempty"`
	
	// Expected result
	Result interface{} `json:"result,omitempty"`
	
	// Flag per undefined result
	UndefinedResult *bool `json:"undefinedResult,omitempty"`
	
	// Error code atteso
	Code *string `json:"code,omitempty"`
	
	// Token dell'errore
	Token *string `json:"token,omitempty"`
}

// TestCase rappresenta un test case processato e pronto per generazione.
type TestCase struct {
	GroupName    string
	CaseNumber   int
	FileName     string
	Expression   string
	InputData    interface{}
	Bindings     map[string]interface{}
	ShouldError  bool
	ErrorCode    string
	ErrorToken   string
	Expected     interface{}
	IsUndefined  bool
	TimeLimit    *int
	DepthLimit   *int
}

// GroupInfo contiene informazioni su un gruppo di test.
type GroupInfo struct {
	Name      string
	Path      string
	TestCount int
	TestCases []TestCase
}

// ImportStats contiene statistiche sull'import.
type ImportStats struct {
	TotalGroups      int
	ProcessedGroups  int
	TotalTestCases   int
	SuccessfulImport int
	FailedImport     int
	Errors           []string
}
