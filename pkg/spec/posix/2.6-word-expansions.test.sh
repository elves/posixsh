#### 2.6.2 Parameter expansion: simple parameter
foo=bar
echo $foo ${foo}
echo $0 $10 ${10}
## argv-json: ["/bin/sh", "1x", "2x", "3x", "4x", "5x", "6x", "7x", "8x", "9x", "10x"]
## STDOUT:
bar bar
/bin/sh 1x0 10x
## END

#### 2.6.2 Parameter expansion: use default values if unset
echo ${unset-default}
null=
echo ${null-default}
foo=bar
echo ${foo-default}
## STDOUT:
default

bar
## END

#### 2.6.2 Parameter expansion: use default values if unset or null
echo ${unset:-default}
null=
echo ${null:-default}
foo=bar
echo ${foo:-default}
## STDOUT:
default
default
bar
## END

#### 2.6.2 Parameter expansion: assign default values if unset
echo ${unset=default}
echo $unset
null=
echo ${null=default}
echo $null
foo=bar
echo ${foo=default}
echo $foo
## STDOUT:
default
default


bar
bar
## END

#### 2.6.2 Parameter expansion: assign default values if unset or null
echo ${unset:=default}
echo $unset
null=
echo ${null:=default}
echo $null
foo=bar
echo ${foo:=default}
echo $foo
## STDOUT:
default
default
default
default
bar
bar
## END

#### 2.6.2 Parameter expansion: use alternative value if set
echo ${unset+alt}
null=
echo ${null+alt}
foo=bar
echo ${foo+alt}
## STDOUT:

alt
alt
## END

#### 2.6.2 Parameter expansion: use alternative value if set and non-null
echo ${unset:+alt}
null=
echo ${null:+alt}
foo=bar
echo ${foo:+alt}
## STDOUT:


alt
## END

#### 2.6.2 Parameter expansion: length
echo ${#unset}
null=
echo ${#null}
foo=bar
echo ${#foo}
## STDOUT:
0
0
3
## END
