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