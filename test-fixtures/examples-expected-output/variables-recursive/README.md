# Recursive variables

This shows an example of variables that reference other variables.

Foo = foo
Bar = foo-bar
Baz = foo-bar-baz

FooList = foo, bar, baz
BarList = foo, bar, baz
FooMap = bar: 2, baz: 3, foo: 1
BarMap = bar: 2, baz: 3, foo: 1

ListWithTemplates = foo, foo-bar, foo-bar-baz
MapWithTemplates = foo: foo, foo-bar: foo-bar, foo-bar-baz: foo-bar-baz
ListWithNestedMap = (name: foo, value: foo), (name: bar, value: bar)
MapWithNestedList = (key: bar, value: 4, 5, 6), (key: foo, value: 1, 2, 3)

IntValue: 42
IntValueInterpolation: 42
FloatValue: 3.14
FloatValueInterpolation: 3.14
BoolValue: true
BoolValueInterpolationSimple: true
BoolValueInterpolationSimpleComplex: true
ListValue: 1, 2, 3
ListValueInterpolation: 1, 2, 3
MapValue: bar: 2, baz: 3, foo: 1
MapValueInterpolation: bar: 2, baz: 3, foo: 1
