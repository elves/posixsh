#### . searches file in PATH when filename doesn't contain slash
mkdir d
echo 'echo sourced file' > d/module
PATH=$PWD/d:$PATH
. module
## stdout: sourced file

#### . doesn't search when filename contains slash
mkdir d
echo 'echo sourced file' > d/module
. d/module
## stdout: sourced file

#### File not found is a fatal error
oldpath=$PATH
PATH=
. module
PATH=$oldpath
echo should not get here
## status: [1, 127]
## stderr-regexp: .+

# Behavior of return within a sourced file is tested in return.test.sh
