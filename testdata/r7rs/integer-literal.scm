; Independently authored check of R7RS-small section 4.1.2.
(import (scheme base))
(write-string (if (= 42 42) "ok" "fail"))
