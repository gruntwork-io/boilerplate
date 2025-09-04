package variables

import (
	"github.com/gruntwork-io/boilerplate/util"
)

// Hook represents a single hook, which is a command that is executed by boilerplate
type Hook struct {
	Env        map[string]string
	Command    string
	Skip       string
	WorkingDir string
	Args       []string
}

// Hooks represents all the scripts to execute as boilerplate hooks
type Hooks struct {
	BeforeHooks []Hook
	AfterHooks  []Hook
}

// MarshalYAML implements the go-yaml marshaler interface so that the config can be marshaled into yaml. We use a custom marshaler
// instead of defining the fields as tags so that we skip the attributes that are empty.
func (hook Hook) MarshalYAML() (any, error) {
	hookYml := map[string]any{}
	if hook.Command != "" {
		hookYml["command"] = hook.Command
	}

	if hook.Skip != "" {
		hookYml["skip"] = hook.Skip
	}

	if len(hook.Args) > 0 {
		hookYml["args"] = hook.Args
	}

	if len(hook.Env) > 0 {
		hookYml["env"] = hook.Env
	}

	if len(hook.WorkingDir) > 0 {
		hookYml["dir"] = hook.WorkingDir
	}

	return hookYml, nil
}
func (hooks Hooks) MarshalYAML() (any, error) {
	hooksYml := map[string]any{}
	// Due to go type system, we can only pass through []interface{}, even though []Hook is technically
	// polymorphic to that type. So we reconstruct the list using the right type before passing it in to the marshal
	// function.
	if len(hooks.BeforeHooks) > 0 {
		interfaceList := []interface{}{}
		for _, hook := range hooks.BeforeHooks {
			interfaceList = append(interfaceList, hook)
		}

		beforeYml, err := util.MarshalListOfObjectsToYAML(interfaceList)
		if err != nil {
			return nil, err
		}

		hooksYml["before"] = beforeYml
	}

	if len(hooks.AfterHooks) > 0 {
		interfaceList := []interface{}{}
		for _, hook := range hooks.AfterHooks {
			interfaceList = append(interfaceList, hook)
		}

		afterYml, err := util.MarshalListOfObjectsToYAML(interfaceList)
		if err != nil {
			return nil, err
		}

		hooksYml["after"] = afterYml
	}

	return hooksYml, nil
}

// UnmarshalHooksFromBoilerplateConfigYaml given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// hooks:
//
//	before:
//	  - command: <CMD>
//	    args:
//	      - <ARG>
//	    env:
//	      <KEY>: <VALUE>
//	    skip: <CONDITION>
//
//	after:
//	  - command: <CMD>
//	    args:
//	      - <ARG>
//	    env:
//	      <KEY>: <VALUE>
//	    skip: <CONDITION>
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
//
//   - command: <CMD>
//     args:
//
//   - <ARG>
//     env:
//     <KEY>: <VALUE>
//
//   - command: <CMD>
//     args:
//
//   - <ARG>
//     env:
//     <KEY>: <VALUE>
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
//
// env:
//
//	<KEY>: <VALUE>
//
// This method unmarshals the YAML data into a Hook struct
func unmarshalHookFromBoilerplateConfigYaml(fields map[string]interface{}, hookName string) (*Hook, error) {
	command, err := unmarshalStringField(fields, "command", true, hookName)
	if err != nil {
		return nil, err
	}

	args, err := UnmarshalListOfStrings(fields, "args")
	if err != nil {
		return nil, err
	}

	env, err := unmarshalMapOfStrings(fields, "env")
	if err != nil {
		return nil, err
	}

	var workingDir string

	dir, err := unmarshalStringField(fields, "dir", false, hookName)
	if err != nil {
		return nil, err
	}

	if dir != nil {
		workingDir = *dir
	}

	skipPtr, err := unmarshalStringField(fields, "skip", false, hookName)
	if err != nil {
		return nil, err
	}

	var skip string
	if skipPtr != nil {
		skip = *skipPtr
	}

	return &Hook{Command: *command, Args: args, Env: env, Skip: skip, WorkingDir: workingDir}, nil
}
