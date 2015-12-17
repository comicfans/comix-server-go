go clone of comix-server (php)

rewrite origin php version (https://raw.githubusercontent.com/song31/comix-server) in go

compare with origin version:

pros:
* no need to run/configure apache/php


cons:
* RAR/CBR not supported
* no encoding conversion


planed feathers:

* runtime cache (fs notify based)
* runtime config
* on the fly image scale (lower bandwidth)

