; Independently authored check of R7RS-small section 6.2.6.
(import (scheme base))
(write-string (if (= (/ 1) 1) "ok" "fail"))
