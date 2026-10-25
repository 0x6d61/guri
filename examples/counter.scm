(import (scheme base) (scheme write))

(define (make-counter)
  (let ((n 0))
    (lambda ()
      (set! n (+ n 1))
      n)))

(define next (make-counter))
(display (next)) (newline)
(display (next)) (newline)
