FROM continuumio/anaconda

RUN pip install pyalgotrade
COPY ./ /root/


ENV SNAME AAPL
ENV SMA 20

WORKDIR /root/

CMD python start.py $SNAME $SMA
