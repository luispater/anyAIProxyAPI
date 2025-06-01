package runner

type Configuration struct {
	Version         string                  `yaml:"version"`
	Name            string                  `yaml:"name"`
	NeedReportToken bool                    `yaml:"need_report_token"`
	Workflow        []ConfigurationWorkflow `yaml:"workflow"`
}

type ConfigurationActions struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Params      []string `yaml:"params"`
}

type ConfigurationWorkflow struct {
	Index       int                           `yaml:"index"`
	Action      string                        `yaml:"action"`
	Description string                        `yaml:"description"`
	Params      []string                      `yaml:"params"`
	Result      []ConfigurationWorkflowResult `yaml:"result"`
	Retry       int                           `yaml:"retry"`
	Failback    *ConfigurationWorkflow        `yaml:"failback"`
	Workflow    []ConfigurationWorkflow       `yaml:"workflow"`
}

type ConfigurationWorkflowResult struct {
	ResultIndex int                                `yaml:"result_index"`
	Name        string                             `yaml:"name"`
	Type        string                             `yaml:"type"`
	Policy      *ConfigurationWorkflowResultPolicy `yaml:"policy"`
}
type ConfigurationWorkflowResultPolicy struct {
	IsTrue   string `yaml:"is_true"`
	IsFalse  string `yaml:"is_false"`
	HasError string `yaml:"has_error"`
	NoError  string `yaml:"no_error"`
}
type ConfigurationWorkflowFailback struct {
	Action      string                        `yaml:"action"`
	Description string                        `yaml:"description"`
	Params      []string                      `yaml:"params"`
	Result      []ConfigurationWorkflowResult `yaml:"result"`
}
