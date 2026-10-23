; Independently authored check of R7RS-small section 6.10.
(import (scheme base))
(write-string (if (= (call-with-values (lambda () (values 5 6)) *) 30) "ok" "fail"))
