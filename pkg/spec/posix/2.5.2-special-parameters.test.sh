#### Special parameter $#
echo $#
## argv-json: ["/bin/sh", "foo", "bar"]
## STDOUT:
2
## END

#### Special parameter $0
echo $0
## argv-json: ["/bin/sh"]
## STDOUT:
/bin/sh
## END

# TODO: Test other special parameters.
