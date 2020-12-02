local funcs = import 'funcdef.libsonnet';

function(boilerplateVars) {
  person: funcs.newPerson(boilerplateVars.Name),
}
