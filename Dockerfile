FROM alpine:3.8
ADD ./kube-entropy .
ADD ./config .
ENTRYPOINT [ "/kube-entropy" ]