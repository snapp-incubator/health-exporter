FROM centos:7

RUN yum install -q -y --enablerepo=extras epel-release && \
    yum install -q -y python python-devel python-pip git unzip

RUN	pip install -q ansible==2.8.4 jinja2==2.11.1 netaddr==0.7.19 \
        ansible-lint==4.2

CMD ["/bin/bash"]
