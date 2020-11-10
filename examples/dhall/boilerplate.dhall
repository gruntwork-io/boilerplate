{-
- Import the boilerplate config types so that they can be used. Note that we use relative import here, but dhall
- supports importing from a URL. In your configuration, replace with:
- https://raw.githubusercontent.com/gruntwork-io/boilerplate/v0.4.0/dhall/types/boilerplate_config.dhall
- TODO: figure out issue with private repos
-
- inputValues represents the boilerplate input values as passed through by the boilerplate runtime. The boilerplate
- runtime will translate the input values yaml to dhall.
-}
let boilerplate = ../../dhall/types/boilerplate_config.dhall

let inputValues = env:BOILERPLATE_DHALL_INPUT_PATH

let fooMapVar
    : boilerplate.variable
    =   boilerplate.emptyVariable
      ⫽ { name = "FooMap"
        , description = "Random map variable foo"
        , type = Some "map"
        }

let bazMapVar
    : boilerplate.variable
    =   boilerplate.emptyVariable
      ⫽ { name = "BazMap"
        , description = "Random map variable baz"
        , type = Some inputValues
        , reference = Some "FooMap"
        }

let config
    : boilerplate.boilerplateConfig
    =   boilerplate.emptyBoilerplateConfig
      ⫽ { variables = Some [ fooMapVar, bazMapVar ] }

in  config
