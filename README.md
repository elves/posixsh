This is xiaq's experimental POSIX shell. If it works out, it will eventually
to be integrated into [Elvish](https://github.com/elves/elvish).

License is BSD 2-clause.

Files in `spec/oil` are derived from the
[Oil](https://github.com/oilshell/oil), licensed under Apache License 2.0. See
[spec/oil/LICENSE](spec/oil/LICENSE).

## Status and limitations

The majority of POSIX shell features are implemented.

The following features are currently missing:

* [ ] Closing FDs in redirections (`<&-`, 2.7.5, 2.7.6)
* [ ] Background jobs and related features
    * [ ] All of 2.9.3 "Async lists"
    * [ ] `$!`
    * [ ] `bg`
    * [ ] `fg`
    * [ ] `jobs`
    * [ ] `wait`
    * [ ] `set -o monitor` (`set -m`)
    * [ ] `set -o notify` (`set -n`)
* [ ] `set -o errexit` (`set -e`)
* [ ] `set -o noexec` (`set -n`)
* [ ] `exec`
* [ ] `getopts`
* [ ] `hash`
* [ ] `$LINENO` (2.5.3)
* [ ] Signal handling
    * [ ] All of 2.11 "Signals and error handling"
    * [ ] `trap`
* [ ] Interactive features
    * [ ] `$ENV` (2.5.3)
    * [ ] `$PS1` (2.5.3)
    * [ ] `$PS2` (2.5.3)
    * [ ] `$PS4` (2.5.3)
    * [ ] `fc`
    * [ ] `set -o ignoreeof`
    * [ ] `set -o nolog`.

Some implemented features are incomplete:

- In `<<-` heredocs, leading tabs in expansions are not stripped (2.7.4). Example:

  ```sh
  cat <<-EOF
    $(echo '
    bar')
  EOF
  ```

  This should print an empty line followed by a line of just `bar`, but
  currently this implementation has a tab before the `bar`.

- Argument of variable expansions may not contain whitespaces (2.6.2). Example:

  ```sh
  echo ${x=foo  bar}
  ```

  This should print `foo bar` (with one space) and assign `$x` to `foo  bar`
  (with two spaces). This implementation treats this as a syntax error now.

Since Go doesn't support `fork`, subshells are run in the same process, with
their own virtualized working directories and variables. This approach has some
inherent limitations:

- Some properties cannot be virtualized: `ulimit`, `umask` and `exec` (when
  implemented) will affect the entire process.

- Code that actually depends on subshells running in separate processes won't
  work correctly.

The following features are out of scope and will likely never be implemented:

- `set -h`.

- `kill` and `newgrp` - both are widely available as standalone commands.

- Internationalization/localization. This includes support for `$LANG`, `$LC_*`
  and `$NLSPATH`.
