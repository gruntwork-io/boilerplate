{-
- Import the boilerplate config types so that they can be used. Note that we use relative import here, but dhall
- supports importing from a URL. In your configuration, replace with:
- https://raw.githubusercontent.com/gruntwork-io/boilerplate/v0.4.0/dhall/types/boilerplate_config.dhall
-}
let boilerplate = ../../dhall/types/boilerplate_config.dhall

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
        , type = Some "map"
        , reference = Some "FooMap"
        }

let config
    : boilerplate.boilerplateConfig
    =   boilerplate.emptyBoilerplateConfig
      ⫽ { variables = Some [ fooMapVar, bazMapVar ] }

in  config
