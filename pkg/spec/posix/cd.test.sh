#### cd changes directory
root=$PWD
mkdir -p d/bin
printf '#!/bin/sh\necho hello\necho save > out-tool' > d/bin/tool
chmod +x d/bin/tool
cd d
# All of the following use the new working directory: command lookup,
# redirection, working directory of the external command
bin/tool > out-redir
cd $root
printf ': %s\n' d/out-*
## STDOUT:
: d/out-redir
: d/out-tool
## END

#### cd sets $PWD and $OLDPWD
root=$PWD
mkdir d1 d2
cd d1
cd ../d2
echo ${PWD#"$root/"} ${OLDPWD#"$root/"}
## stdout: d2 d1

#### cd with no argument changes to $HOME
root=$PWD
mkdir home
HOME=$root/home
cd
echo ${PWD#"$root/"}
## stdout: home

#### cd - changes to $OLDPWD and prints the working directory
root=$PWD
mkdir d1 d2
cd d1
cd ../d2
cd -
echo ${PWD#"$root/"}
## stdout-regexp: .+/d1\nd1\n

#### cd tries paths in CDPATH for relative paths
root=$PWD
mkdir -p d1 d2/bin
CDPATH=$PWD/d1:$PWD/d2
cd bin
echo ${PWD#"$root/"}
## stdout: d2/bin

#### cd doesn't use CDPATH for relative paths starting with .. or .
root=$PWD
mkdir -p bin d1 d2/bin
CDPATH=$PWD/d1:$PWD/d2
cd ./bin # will be in d2/bin if CDPATH is consulted
echo ${PWD#"$root/"}
cd ../bin # will be in d2/bin if CDPATH is consulted
echo ${PWD#"$root/"}
## STDOUT:
bin
bin
## END

#### cd -L processes .. components before resolving symbolic links
root=$PWD
mkdir -p d1/d2
ln -s $PWD/d1/d2 d1/d2/d3
cd -L d1/d2/d3/.. # d1/d2/d3/.. becomes just d1/d2
echo ${PWD#"$root/"}
## stdout: d1/d2

#### cd -P doesn't process .. components before resolving symbolic links
root=$PWD
mkdir -p d1/d2
ln -s $PWD/d1/d2 d1/d2/d3
cd -P d1/d2/d3/.. # d3 -> d2, d2/.. is d1
echo ${PWD#"$root/"}
## stdout: d1

#### cd is cd -L by default
root=$PWD
mkdir -p d1/d2
ln -s $PWD/d1/d2 d1/d2/d3
cd d1/d2/d3/.. # d1/d2/d3/.. becomes just d1/d2
echo ${PWD#"$root/"}
## stdout: d1/d2
