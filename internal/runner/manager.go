package runner

import (
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/luispater/anyAIProxyAPI/internal/config"
	"github.com/luispater/anyAIProxyAPI/internal/method"
	"github.com/playwright-community/playwright-go"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type RunnerResult struct {
	Value any
	Type  string
}

type RunnerManager struct {
	name            string
	page            *playwright.Page
	configs         map[string]Configuration
	method          *method.Method
	results         map[string]RunnerResult
	debug           bool
	debugConfigs    map[string]string
	appConfigRunner config.AppConfigRunner
	abort           bool
}

func NewRunnerManager(name string, appConfigRunner config.AppConfigRunner, page *playwright.Page, debug bool) (*RunnerManager, error) {
	runner := &RunnerManager{
		name:            name,
		page:            page,
		method:          method.NewMethod(page),
		configs:         make(map[string]Configuration),
		results:         make(map[string]RunnerResult),
		debug:           debug,
		debugConfigs:    make(map[string]string),
		appConfigRunner: appConfigRunner,
	}
	err := runner.LoadConfigurations()
	if err != nil {
		return nil, err
	}
	return runner, nil
}

func (rm *RunnerManager) Abort() {
	rm.abort = true
}

func (rm *RunnerManager) SetVariable(name string, value any, valueType string) {
	rm.results[name] = RunnerResult{
		Value: value,
		Type:  valueType,
	}
}

func (rm *RunnerManager) NeedReportToken(name string) bool {
	cfg, ok := rm.configs[name]
	if !ok {
		return false
	}
	return cfg.NeedReportToken
}

func (rm *RunnerManager) GetTokenReport() (uint64, uint64, uint64) {
	for {
		var promptTokens, completionTokens, totalTokens uint64

		if pt, hasKey := rm.results["PromptTokens"]; hasKey {
			promptTokens = pt.Value.(uint64)
		} else {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if ct, hasKey := rm.results["CompletionTokens"]; hasKey {
			completionTokens = ct.Value.(uint64)
		} else {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if tt, hasKey := rm.results["TotalTokens"]; hasKey {
			totalTokens = tt.Value.(uint64)
		} else {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return promptTokens, completionTokens, totalTokens
	}
}

func (rm *RunnerManager) LoadConfiguration(name, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg Configuration
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return err
	}

	rm.configs[name] = cfg
	return nil
}

// LoadConfigurations scans all yaml files in the runner directory and calls LoadConfiguration method by filename
func (rm *RunnerManager) LoadConfigurations() error {
	// Scan all yaml and yml files in the runner directory
	yamlFiles, err := filepath.Glob(fmt.Sprintf("runner/%s/*.yaml", rm.name))
	if err != nil {
		return fmt.Errorf("failed to scan yaml files: %v", err)
	}

	ymlFiles, err := filepath.Glob(fmt.Sprintf("runner/%s/*.yml", rm.name))
	if err != nil {
		return fmt.Errorf("failed to scan yml files: %v", err)
	}

	// Merge two file lists
	allFiles := append(yamlFiles, ymlFiles...)

	// Iterate through all found files
	for _, filePath := range allFiles {
		// Get filename (without path and extension)
		fileName := filepath.Base(filePath)
		// Remove extension
		name := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		switch name {
		case rm.appConfigRunner.Init:
			name = "init"
		case rm.appConfigRunner.ChatCompletions:
			name = "chat_completions"
		case rm.appConfigRunner.ContextCanceled:
			name = "context_canceled"
		}

		log.Debugf("Loading configuration file: %s -> %s", name, filePath)

		// Call LoadConfiguration method
		err = rm.LoadConfiguration(name, filePath)
		if err != nil {
			log.Debugf("Failed to load configuration file %s: %v", filePath, err)
			return fmt.Errorf("failed to load configuration file %s: %v", filePath, err)
		}

		log.Debugf("Successfully loaded configuration file: %s", name)
		if rm.debug {
			rm.debugConfigs[name] = filePath
		}
	}

	log.Debugf("Total loaded %d configuration files", len(allFiles))
	return nil
}

func (rm *RunnerManager) Run(name string) error {
	if rm.debug {
		err := rm.LoadConfigurations()
		if err != nil {
			log.Debugf("Load configuare files failed: %v", err)
		}
	}
	cfg, ok := rm.configs[name]
	if !ok {
		return nil
	}
	return rm.runWorkflow(0, cfg.Workflow, []int{})
}

func (rm *RunnerManager) runWorkflow(level int, workflows []ConfigurationWorkflow, doWorkflowIndex []int) error {
	executeIndex := 0
	retry := -1

outLoop:
	for executeIndex < len(workflows) {
		if rm.abort {
			log.Debugf("Get abort signal, stop all workflow")
			return nil
		}

		if len(doWorkflowIndex) > 0 {
			if !inArray(workflows[executeIndex].Index, doWorkflowIndex) {
				log.Debugf("execute workflow Index: %d, level: %d not in doWorkflowIndex: %v, skip", workflows[executeIndex].Index, level, doWorkflowIndex)
				executeIndex++
				continue
			}
		}

		if v, hasKey := rm.results["SKIP-GLOBAL-WORKFLOW-IDX"]; hasKey {
			if v.Value != nil && level == 0 {
				if inArray(workflows[executeIndex].Index, v.Value) {
					log.Debugf("execute workflow Index: %d, level: %d in SKIP-GLOBAL-WORKFLOW-IDX: %v, skip", workflows[executeIndex].Index, level, rm.results["SKIP-GLOBAL-WORKFLOW-IDX"].Value)
					executeIndex++
					continue
				}
			}
		}

		log.Debugf("execute workflow loop index: %d, Index: %d, level: %d", executeIndex, workflows[executeIndex].Index, level)
		if retry == -1 {
			retry = workflows[executeIndex].Retry
		} else {
			retry--
		}
		if retry < 0 {
			return fmt.Errorf("retry failed")
		}

		if workflows[executeIndex].Action == "DoRunner" {
			r, err := NewRunnerManager(rm.name, rm.appConfigRunner, rm.page, rm.debug)
			if err != nil {
				log.Error(err)
				return err
			}
			for k, v := range rm.results {
				r.SetVariable(k, v.Value, v.Type)
			}

			err = r.Run(workflows[executeIndex].Params[0])
			if err != nil {
				log.Error(err)
				if err.Error() != "break" {
					return err
				}
			}
			executeIndex++
			retry = -1 // clear retry count
			continue outLoop
		} else if workflows[executeIndex].Action == "ReportToken" {

		}

		workflow := workflows[executeIndex]
		log.Debugf("prepare to execute workflow %s", workflow.Action)

		callParams := make([]interface{}, len(workflow.Params))
		for i := 0; i < len(workflow.Params); i++ {
			callParams[i] = workflow.Params[i]
		}

		log.Debugf("prepare to execute workflow %s with params %v", workflow.Action, workflow.Params)
		results, err := rm.executeMethod(rm.method, workflow.Action, callParams)
		if err != nil {
			log.Debugf("execute workflow %s failed: %v", workflow.Action, err)
			if errFailback := rm.executeFallback(workflow.Failback); errFailback != nil {
				return err
			} else {
				continue
			}
		}

		doWorkflow := false
		doFailback := false
		arrayWorkflowIndex := make([]int, 0)
		for _, result := range workflow.Result {
			if result.Name != "" {
				log.Debugf("store result to global variable #%s#", result.Name)
				rm.results[result.Name] = RunnerResult{
					Value: results[result.ResultIndex].Interface(),
					Type:  result.Type,
				}
			}
			rule := ""
			if result.Type == "bool" {
				if v, ok := results[result.ResultIndex].Interface().(bool); ok {
					if v {
						if result.Policy.IsTrue != "" {
							rule = result.Policy.IsTrue
						}
					} else {
						if result.Policy.IsFalse != "" {
							rule = result.Policy.IsFalse
						}
					}
				} else {
					log.Debugf("Configuration error, return value %d is not bool type", result.ResultIndex)
				}
			} else if result.Type == "error" {
				if !results[result.ResultIndex].IsNil() {
					if result.Policy.HasError != "" {
						rule = result.Policy.HasError
					}
				} else {
					if result.Policy.NoError != "" {
						rule = result.Policy.NoError
					}
					log.Debugf("Result error is nil, successfully execute workflow %s", workflow.Action)
				}
			}

			if rule != "" {
				log.Debugf("rule is %s", rule)
				if rule == "CONTINUE" {
					executeIndex++
					retry = -1 // clear retry count
					continue outLoop
				} else if rule == "DO-WORKFLOW" {
					doWorkflow = true
				} else if rule == "FAILBACK" {
					doFailback = true
				} else if rule == "FAILED" {
					return fmt.Errorf("workflow failed")
				} else if strings.HasPrefix(rule, "DO-WORKFLOW-IDX:") {
					log.Debugf("DO-WORKFLOW-IDX: %s", rule[16:])
					// Handle DO-WORKFLOW-IDX:1,2,3 cases
					idxs := strings.Split(rule[16:], ",")
					for _, idx := range idxs {
						i, errAtoi := strconv.Atoi(idx)
						if errAtoi != nil {
							log.Warnf("configuration error, workflow index is not a number: %v", errAtoi)
						} else {
							arrayWorkflowIndex = append(arrayWorkflowIndex, i)
						}
					}
					log.Debugf("processed DO-WORKFLOW-IDX: %v", arrayWorkflowIndex)
					doWorkflow = true
				} else if rule == "LOOP" {
					retry = retry + 1 // clear retry count
					continue outLoop
				} else if rule == "LOOP-WORKFLOW" { // loop all the workflow
					retry = retry + 1 // clear retry count
					executeIndex = 0
					continue outLoop
				} else if rule == "LOOP-PARENT" { // loop all the parent's workflow
					return fmt.Errorf("loop parent")
				} else if rule == "BREAK" {
					break outLoop
				} else if strings.HasPrefix(rule, "SKIP-GLOBAL-WORKFLOW-IDX:") {
					log.Debugf("SKIP-GLOBAL-WORKFLOW-IDX: %s", rule[25:])
					// Handle SKIP-GLOBAL-WORKFLOW-IDX:1,2,3 cases
					arraySkipGlobalWorkflowIndex := make([]int, 0)
					idxs := strings.Split(rule[25:], ",")
					for _, idx := range idxs {
						i, errAtoi := strconv.Atoi(idx)
						if errAtoi != nil {
							log.Warnf("configuration error, workflow index is not a number: %v", errAtoi)
						} else {
							arraySkipGlobalWorkflowIndex = append(arraySkipGlobalWorkflowIndex, i)
						}
					}
					log.Debugf("skip global workflow index: %v", arraySkipGlobalWorkflowIndex)
					rm.SetVariable("SKIP-GLOBAL-WORKFLOW-IDX", arraySkipGlobalWorkflowIndex, "array")
				}
			} else {
				log.Debugf("no rule, continue to next result check.")
			}
		}

		if doFailback {
			err = rm.executeFallback(workflow.Failback)
			if err == nil {
				continue
			} else if err.Error() == "break" {
				break
			} else {
				return err
			}
		}

		if doWorkflow {
			err = rm.runWorkflow(level+1, workflow.Workflow, arrayWorkflowIndex)
			if err != nil {
				if err.Error() == "loop parent" {
					retry = retry + 1 // clear retry count
					continue outLoop
				} else if err.Error() == "workflow failed" {
					return err
				}

				// sub workflow failed, need to retry
				continue
			}
		}

		executeIndex++
		retry = -1 // clear retry count

		log.Debugf("next execute workflow Index: %d, total workflows: %d", workflow.Index, len(workflows))
		continue
	}

	return nil

}

func (rm *RunnerManager) executeFallback(failback *ConfigurationWorkflow) error {
	workflow := failback
	log.Debugf("prepare to execute failback %s", workflow.Action)

	callParams := make([]interface{}, len(workflow.Params))
	for i := 0; i < len(workflow.Params); i++ {
		callParams[i] = workflow.Params[i]
	}

	log.Debugf("prepare to execute failback %s with params %v", workflow.Action, workflow.Params)
	results, err := rm.executeMethod(rm.method, workflow.Action, callParams)
	if err != nil {
		log.Warnf("execute failback %s failed: %v", workflow.Action, err)
		if errFailback := rm.executeFallback(workflow.Failback); errFailback != nil {
			return err
		}
	}

	for _, result := range workflow.Result {
		if result.Name != "" {
			rm.results[result.Name] = RunnerResult{
				Value: results[result.ResultIndex].Interface(),
				Type:  result.Type,
			}
		}
		rule := ""
		if result.Type == "bool" {
			if v, ok := results[result.ResultIndex].Interface().(bool); ok {
				if v {
					if result.Policy.IsTrue != "" {
						rule = result.Policy.IsTrue
					}
				} else {
					if result.Policy.IsFalse != "" {
						rule = result.Policy.IsFalse
					}
				}
			} else {
				log.Warnf("Configuration error, return value %d is not bool type", result.ResultIndex)
			}
		} else if result.Type == "error" {
			if !results[result.ResultIndex].IsNil() {
				if result.Policy.HasError != "" {
					rule = result.Policy.HasError
				}
			} else {
				if result.Policy.NoError != "" {
					rule = result.Policy.NoError
				}
			}
		}

		if rule != "" {
			switch rule {
			case "FAILED":
				return fmt.Errorf("failback failed")
			case "BREAK":
				return fmt.Errorf("break")
			}
		}
	}
	return nil
}

func (rm *RunnerManager) executeMethod(obj any, methodName string, params []interface{}) ([]reflect.Value, error) {
	methodValue := reflect.ValueOf(obj)
	methodType := reflect.TypeOf(obj)

	m, found := methodType.MethodByName(methodName)
	if !found {
		return nil, fmt.Errorf("method '%s' not found", methodName)
	}

	log.Debugf("execute method: %s", methodName)

	// Get method type
	methodFunc := m.Type

	if methodFunc.NumIn() != len(params)+1 {
		return nil, fmt.Errorf("parameter number not match")
	}

	// Prepare parameters (skip the first parameter, i.e., receiver)
	args := make([]reflect.Value, methodFunc.NumIn()-1)

	// Get parameters one by one
	for i := 1; i < methodFunc.NumIn(); i++ {
		paramType := methodFunc.In(i)
		paramIndex := i - 1

		// Special handling for playwright.Page type, no user input required
		if paramType.String() == "playwright.Page" {
			args[paramIndex] = reflect.ValueOf(rm.page)
			continue
		}
		if reflect.TypeOf(params[i-1]).Kind() == reflect.String {
			input := strings.TrimSpace(reflect.ValueOf(params[i-1]).String())
			if len(input) > 2 && input[0] == '#' && input[len(input)-1] == '#' {
				if input == "#NEW_RUNNER#" {
					newRm, _ := NewRunnerManager(rm.name, rm.appConfigRunner, rm.page, rm.debug)
					args[paramIndex] = reflect.ValueOf(newRm)
				} else {
					input = input[1 : len(input)-1]
					args[paramIndex] = reflect.ValueOf(rm.results[input].Value)
				}
			} else {
				// Try to convert parameter type
				value, err := rm.convertToType(input, paramType)
				if err != nil {
					return nil, fmt.Errorf("parameter type error: %v", err)
				}
				args[paramIndex] = value
			}
		} else {
			args[paramIndex] = reflect.ValueOf(params[i-1])
		}

	}

	// Execute method
	results := methodValue.MethodByName(methodName).Call(args)

	// Check return values
	return rm.handleResults(results)
}

// convertToType converts string input to the specified type
func (rm *RunnerManager) convertToType(input string, targetType reflect.Type) (reflect.Value, error) {
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(input), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot convert to integer: %v", err)
		}
		return reflect.ValueOf(val).Convert(targetType), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(input, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot convert to unsigned integer: %v", err)
		}
		return reflect.ValueOf(val).Convert(targetType), nil

	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot convert to float: %v", err)
		}
		return reflect.ValueOf(val).Convert(targetType), nil

	case reflect.Bool:
		val, err := strconv.ParseBool(input)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot convert to boolean: %v", err)
		}
		return reflect.ValueOf(val), nil

	default:
		// Handle special types, such as playwright.Page
		if targetType.String() == "playwright.Page" {
			// Automatically use current page instance, no user input required
			log.Debugf("(automatically using current page instance)")
			return reflect.ValueOf(rm.page), nil
		}
		return reflect.Value{}, fmt.Errorf("unsupported parameter type: %s", targetType.String())
	}
}

// handleResults handles the return values of method execution
func (rm *RunnerManager) handleResults(results []reflect.Value) ([]reflect.Value, error) {
	if len(results) > 0 {
		return results, nil
	} else {
		log.Debugf("execute successfully, but no return value")
	}
	return nil, nil
}

func inArray(needle interface{}, hystack interface{}) bool {
	switch key := needle.(type) {
	case string:
		for _, item := range hystack.([]string) {
			if key == item {
				return true
			}
		}
	case int:
		for _, item := range hystack.([]int) {
			if key == item {
				return true
			}
		}
	case int64:
		for _, item := range hystack.([]int64) {
			if key == item {
				return true
			}
		}
	default:
		return false
	}
	return false
}
