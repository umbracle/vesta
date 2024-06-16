local input = {};

local gen(func) =
  if std.length(input) == 0 then
    { no_generated: 'true' }
  else
    func(input)
;

{
  match:: function(input) {},
  task: gen(self.match),
}
