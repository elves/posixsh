#### Simple parameter
foo=bar
echo $foo ${foo}
echo $0 $10 ${10}
## argv-json: ["/bin/sh", "1x", "2x", "3x", "4x", "5x", "6x", "7x", "8x", "9x", "10x"]
## STDOUT:
bar bar
/bin/sh 1x0 10x
## END

#### Parameter with escaped or quoted right braces
echo ${foo-\}} ${foo-"}"}
## stdout: } }

#### Use default values if unset (-)
echo ${unset-default}
null=
echo ${null-default}
foo=bar
echo ${foo-default}
## STDOUT:
default

bar
## END

#### Use default values if unset or null (:-)
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

#### Assign default values if unset (=)
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

#### Assign default values if unset or null (:=)
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

#### Use alternative value if set (+)
echo ${unset+alt}
null=
echo ${null+alt}
foo=bar
echo ${foo+alt}
## STDOUT:

alt
alt
## END

#### Use alternative value if set and non-null (:+)
echo ${unset:+alt}
null=
echo ${null:+alt}
foo=bar
echo ${foo:+alt}
## STDOUT:


alt
## END

#### Substitution operators only evaluate arguments when needed
foo=bar
echo ${foo-`exit 1`}
## stdout: bar

#### Substitution operators respect the quoting of arguments
printf ': %s\n' ${foo-"a b c"}
## STDOUT:
: a b c
## END

#### Argument value of substitution operator undergoes field splitting
x='a b c'
printf ': %s\n' ${foo-$x}
## STDOUT:
: a
: b
: c
## END

#### Argument value of substitution operator undergoes pathname expansion
x='*'
touch bar foo
printf ': %s\n' ${foo-$x}
## STDOUT:
: bar
: foo
## END

#### Argument value of assignment operators is preserved without field splitting in variable value
x='a b c'
printf ': %s\n' ${foo=$x}
printf ': %s\n' "$foo"
## STDOUT:
: a
: b
: c
: a b c
## END

#### Argument value of assignment operators is preserved without pathname expansion in variable value
x='*'
touch bar foo
printf ': %s\n' ${foo=$x}
printf ': %s\n' "$foo"
## STDOUT:
: bar
: foo
: *
## END

#### Length (#)
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

#### Remove smallest suffix pattern (%)
x='pa apa ap'
echo "${x%p*}"
## stdout: pa apa a

#### Remove largest suffix pattern (%%)
x='pa apa ap'
echo "${x%%p*}"
## stdout:

#### Remove smallest prefix pattern (#)
x='pa apa ap'
echo "${x#*p}"
## stdout: a apa ap

#### Remove largest prefix pattern (##)
x='pa apa ap'
echo "${x##*p}"
## stdout:
