; R7RS-small 4.1.3: either operand order is permitted.
(import (scheme base))
(let ((n 0))
  (define (record x) (set! n (+ (* n 10) x)) x)
  (+ (record 1) (record 2))
  (write-string (number->string n)))
