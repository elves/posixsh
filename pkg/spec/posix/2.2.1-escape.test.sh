#### Escaping special characters with backslash
echo \|\&\;\<\>\(\)\$\`\\\"\'\ \	\*\?\[\#\~\=\%
## stdout-json: "|&;<>()$`\\\"' \t*?[#~=%\n"

#### Line continuation
e\
cho foo
## STDOUT:
foo
## END
