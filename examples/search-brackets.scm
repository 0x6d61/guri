(import (scheme base) (scheme write))

;; Return a vector of matching positions; non-bracket entries remain #f.
;; Indices count Unicode scalar values, as string-ref does.
(define (search-brackets code)
  (let ((stack '())
        (brackets (make-vector (string-length code) #f)))
    (do ((pc 0 (+ pc 1)))
        ((= pc (string-length code))
         (if (null? stack)
             brackets
             (error "[が閉じられていません。")))
      (case (string-ref code pc)
        ((#\[) (set! stack (cons pc stack)))
        ((#\])
         (if (null? stack)
             (error "] に対応する [ がありません。" pc)
             (let ((left (car stack)))
               (vector-set! brackets left pc)
               (vector-set! brackets pc left)
               (set! stack (cdr stack)))))))))

(define pairs (search-brackets "[[]]"))
(write pairs) (newline) ; #(3 2 1 0)
