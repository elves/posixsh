#### Redirect standard input for both reading and writing
echo content > file
cat <> file
## stdout: content

#### Redirect FD for both reading and writing
echo content 1<> file
cat file
## stdout: content

# TODO: Test a command that both read and write
