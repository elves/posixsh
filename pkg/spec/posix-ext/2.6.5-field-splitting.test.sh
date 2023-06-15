#### Word ending with IFS delimiter doesn't produce a final empty field

# When a word that ends with IFS delimiter undergoes field splitting, all of
# dash, bash and ksh don't produce a final empty field.
# 
# POSIX doesn't specify this behavior explicitly, although it refers to IFS
# delimiters as "field terminators" in one place, which seems to hint at this
# behavior: if the final delimiter merely terminates the previous field, there
# is no final empty field.
# 
# Notably, zsh does produce a final empty field in this case.
# 
# We follow the more popular behavior.
IFS=:
x=:a::b:
printf ': %s\n' $x
## STDOUT:
: 
: a
: 
: b
## END
