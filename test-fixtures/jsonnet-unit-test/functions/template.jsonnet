local newPerson(name) = {
  name: name,
};

function(boilerplateVars) {
  person: newPerson(boilerplateVars.Name),
}
