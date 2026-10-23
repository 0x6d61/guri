; Independently authored check of R7RS-small section 6.9.
(import (scheme base))
(write-string (if (let ((b (bytevector 1 2))) (bytevector-u8-set! b 0 255) (= (bytevector-u8-ref b 0) 255)) "ok" "fail"))
