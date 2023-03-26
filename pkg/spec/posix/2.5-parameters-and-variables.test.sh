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

#### 2.5.2 Special parameters: $#
echo $#
## argv-json: ["/bin/sh", "foo", "bar"]
## STDOUT:
2
## END

#### 2.5.2 Special parameters: $0
echo $0
## argv-json: ["/bin/sh"]
## STDOUT:
/bin/sh
## END
