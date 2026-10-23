; Independently authored check of R7RS-small section 6.7.
(import (scheme base))
(write-string (if (let ((s (string-copy "abc"))) (string-set! s 1 #\z) (string=? s "azc")) "ok" "fail"))
