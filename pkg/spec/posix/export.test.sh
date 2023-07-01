#### Exporting a variable to external commands
export foo=bar
sh -c 'echo $foo'
## stdout: bar

#### Exportinging multiple variables
export foo=bar lorem=ipsum
sh -c 'echo $foo $lorem'
## stdout: bar ipsum

#### Exporting a variable without assigning to it
foo=bar
export foo
echo $foo
## stdout: bar

#### Exporting an unset variable doesn't set it
export foo
echo ${foo-unset}
## stdout: unset

#### export -p
export foo=bar
export -p
## stdout-regexp: (?m).*^export foo=bar$.*

#### export -p, exported but unset variable
export foo
export -p
## stdout-regexp: (?m).*^export foo$.*
