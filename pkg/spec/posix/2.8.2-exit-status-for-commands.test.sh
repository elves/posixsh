#### Command not found error exits with 127
PATH=
bad-command
## status: 127

# TODO: Enable when implemented
#
# #### Command not executable error exits with 126
# PATH=$PWD
# touch foo
# foo
# ## status: 126

# TODO: Enable when there's support for asserting status is between 1 and 125
#
# #### Word expansion error exits with a status between 1 and 125
# echo $(( 1//2 ))
# ## status: 1-125
# 
# #### Redirection error exits with a status between 1 and 125
# echo < non-existent
# ## status: 1-125

# TODO: Test that the status is retrieved with WEXITSTATUS

# TODO: Test that $? reports the full eight bits

# TODO: Test signal exit status > 128
