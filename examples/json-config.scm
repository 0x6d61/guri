(import (scheme base) (scheme file) (scheme process-context) (srfi 180))

(define args (cdr (command-line)))
(unless (= (length args) 2)
  (error "usage: json-config.scm INPUT.json OUTPUT.json"))

(define config
  (call-with-input-file (car args)
    (lambda (port)
      (parameterize ((json-nesting-depth-limit 64)
                     (json-number-of-character-limit 1000000))
        (let ((value (json-read port)))
          (when (eof-object? value) (error "empty configuration"))
          (unless (eof-object? (json-read port))
            (error "extra data after configuration"))
          (unless (list? value) (error "configuration must be a JSON object"))
          value)))))

(define previous (assq 'runs config))
(define runs (if previous (cdr previous) 0))
(unless (and (exact-integer? runs) (>= runs 0))
  (error "runs must be a nonnegative integer"))

(define updated
  (cons (cons 'runs (+ runs 1))
    (let loop ((rest config) (out '()))
      (cond ((null? rest) (reverse out))
            ((eq? (caar rest) 'runs) (loop (cdr rest) out))
            (else (loop (cdr rest) (cons (car rest) out)))))))

(call-with-output-file (cadr args)
  (lambda (port) (json-write updated port) (newline port)))
(json-write updated)
(newline)
