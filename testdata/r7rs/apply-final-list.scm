; Independently authored check of R7RS-small section 6.10.
(import (scheme base))
(write-string (if (= (apply + 1 '(2 3)) 6) "ok" "fail"))
