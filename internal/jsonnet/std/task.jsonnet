local input = std.native('input')();

local gen(name, func) =
  if std.length(input) == 0 then
    { no_generated: 'true' }
  else
    if input.name == name then
      func(input.params)
    else
      { no_generated: 'true' }
;

{
  generate:: function(input) {},
  task: gen(self.name, self.generate),
}
