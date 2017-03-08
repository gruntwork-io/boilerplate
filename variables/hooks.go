package variables

// A single hook, which is a command that is executed by boilerplate
type Hook struct {
	Command string
	Args    []string
	Env     map[string]string
}

// All the scripts to execute as boilerplate hooks
type Hooks struct {
	BeforeHooks []Hook
	AfterHooks  []Hook
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// hooks:
//   before:
//     - command: <CMD>
//       args:
//         - <ARG>
//       env:
//         <KEY>: <VALUE>
//
//   after:
//     - command: <CMD>
//       args:
//         - <ARG>
//       env:
//         <KEY>: <VALUE>
//
// This method takes the data above and unmarshals it into a Hooks struct
func UnmarshalHooksFromBoilerplateConfigYaml(fields map[string]interface{}) (Hooks, error) {
	hookFields, err := unmarshalMapOfFields(fields, "hooks")
	if err != nil {
		return Hooks{}, err
	}

	beforeHooks, err := unmarshalHooksFromBoilerplateConfigYaml(hookFields, "before")
	if err != nil {
		return Hooks{}, err
	}

	afterHooks, err := unmarshalHooksFromBoilerplateConfigYaml(hookFields, "after")
	if err != nil {
		return Hooks{}, err
	}

	return Hooks{BeforeHooks: beforeHooks, AfterHooks: afterHooks}, nil
}

// Given a list of key:value pairs read from a Boilerplate YAML config file of the format:
//
// hookName:
//   - command: <CMD>
//     args:
//       - <ARG>
//     env:
//       <KEY>: <VALUE>
//
//   - command: <CMD>
//     args:
//       - <ARG>
//     env:
//       <KEY>: <VALUE>
//
// This method takes looks up the given hookName in the map and unmarshals the data inside of it it into a list of
// Hook structs
func unmarshalHooksFromBoilerplateConfigYaml(fields map[string]interface{}, hookName string) ([]Hook, error) {
	hookFields, err := unmarshalListOfFields(fields, hookName)
	if err != nil || hookFields == nil {
		return nil, err
	}

	hooks := []Hook{}

	for _, hookField := range hookFields {
		hook, err := unmarshalHookFromBoilerplateConfigYaml(hookField, hookName)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, *hook)
	}

	return hooks, nil
}

// Given key:value pairs read from a Boilerplate YAML config file of the format:
//
// command: <CMD>
// args:
//   - <ARG>
// env:
//   <KEY>: <VALUE>
//
// This method unmarshals the YAML data into a Hook struct
func unmarshalHookFromBoilerplateConfigYaml(fields map[string]interface{}, hookName string) (*Hook, error) {
	command, err := unmarshalStringField(fields, "command", true, hookName)
	if err != nil {
		return nil, err
	}

	args, err := unmarshalListOfStrings(fields, "args")
	if err != nil {
		return nil, err
	}

	env, err := unmarshalMapOfStrings(fields, "env")
	if err != nil {
		return nil, err
	}

	return &Hook{Command: *command, Args: args, Env: env}, nil
}
