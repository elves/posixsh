#### 2.2.1 Escaping special characters with backslash
echo \|\&\;\<\>\(\)\$\`\\\"\'\ \	\*\?\[\#\~\=\%
## stdout-json: "|&;<>()$`\\\"' \t*?[#~=%\n"

#### 2.2.1 Line continuation
e\
cho foo
## STDOUT:
foo
## END

#### 2.2.2 Single quotes
echo 'a\
b'
## STDOUT:
a\
b
## END

#### 2.2.3 Double quotes: variable
foo=bar
echo "foo=$foo"
echo "foo=${foo}"
## STDOUT:
foo=bar
foo=bar
## END

#### 2.2.3 Double quotes: command subtitution with $
echo "output=$(echo "foo")"
## STDOUT:
output=foo
## END

#### 2.2.3 Double quotes: arithmetic expansion
echo "answer=$(( 7 * 6 ))"
## STDOUT:
answer=42
## END

#### 2.2.3 Double quotes: command substitution with `
echo "output=`echo foo`"
## STDOUT:
output=foo
## END
