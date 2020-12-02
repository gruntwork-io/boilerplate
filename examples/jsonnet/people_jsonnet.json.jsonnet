function(boilerplateVars) {
  local personAge = {
    age: boilerplateVars.AgeMap[boilerplateVars.Name],
  },

  people: [
    {
      name: boilerplateVars.Name,
      favoriteFoods: boilerplateVars.FavoriteFoods,
    } + (
      if boilerplateVars.IncludeAge then personAge else {}
    ),
  ]
}
