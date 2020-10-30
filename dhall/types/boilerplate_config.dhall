{- Defines types that are useful for generating boilerplate configs.
- The emptyXXX variables are useful for constructing new records of those types. The intention is to use them with `//`
- to update the value. For example, to define a new variable:
- let fooVar = emptyVariable // { name = "foo", description = "Variable for foo" }
-}

let Variable
    : Type
    = { name : Text
      , description : Text
      , type : Optional Text
      , default : Optional Text
      , options : Optional (List Text)
      , reference : Optional Text
      }

let emptyVariable
    : Variable
    = { name = ""
      , description = ""
      , type = None Text
      , default = None Text
      , options = None (List Text)
      , reference = None Text
      }

let Dependency
    : Type
    = { name : Text
      , templateUrl : Text
      , outputFolder : Text
      , skip : Optional Text
      , dontInheritVariables : Optional Bool
      , variables : Optional (List Variable)
      }

let emptyDependency
    : Dependency
    = { name = ""
      , templateUrl = ""
      , outputFolder = ""
      , skip = None Text
      , dontInheritVariables = None Bool
      , variables = None (List Variable)
      }

let MapItem
    : Type
    = { mapKey : Text, mapValue : Text }

let Hook
    : Type
    = { command : Text
      , args : List Text
      , envVars : Optional (List MapItem)
      , skip : Optional Text
      }

let emptyHook
    : Hook
    = { command = ""
      , args = [] : List Text
      , envVars = None (List MapItem)
      , skip = None Text
      }

let HookConfig
    : Type
    = { before : Optional (List Hook), after : Optional (List Hook) }

let emptyHookConfig
    : HookConfig
    = { before = None (List Hook), after = None (List Hook) }

let BoilerplateConfig
    : Type
    = { variables : Optional (List Variable)
      , dependencies : Optional (List Dependency)
      , hooks : Optional HookConfig
      , partials : Optional (List Text)
      }

let emptyBoilerplateConfig
    : BoilerplateConfig
    = { variables = None (List Variable)
      , dependencies = None (List Dependency)
      , hooks = None HookConfig
      , partials = None (List Text)
      }

in  { variable = Variable
    , dependency = Dependency
    , hook = Hook
    , hookConfig = HookConfig
    , boilerplateConfig = BoilerplateConfig
    , emptyVariable
    , emptyDependency
    , emptyHook
    , emptyHookConfig
    , emptyBoilerplateConfig
    }
