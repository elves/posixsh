#### 2.5.1 Positional parameters from initial argv
echo $2 $1 ${1} ${2}
## argv-json: ["/bin/sh", "foo", "bar"]
## STDOUT:
bar foo foo bar
## END

#### 2.5.1 Positional parameters in functions
f() { echo $2 $1 }
f foo bar
## STDOUT:
bar foo
## END

#### 2.5.1 Positional parameters from set
set -- foo bar
echo $2 $1
## STDOUT:
bar foo
## END
