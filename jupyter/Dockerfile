FROM python:3.9

RUN apt-get update -y \
    && apt-get upgrade -y

RUN curl -sL https://deb.nodesource.com/setup_12.x |bash - \
    && apt-get install -y --no-install-recommends \
    wget \
    git \
    vim \
    curl \
    make \
    cmake \
    nodejs \
    && apt-get autoremove -y \
    && apt-get clean \
    && rm -rf \
        /var/lib/apt/lists/* \
        /var/cache/apt/* \
        /usr/local/src/* \
        /tmp/*

# install python library
COPY requirements.txt .
RUN pip3 install --upgrade pip && \
    pip3 install --no-cache-dir -r requirements.txt \
    && rm -rf ~/.cache/pip

# install jupyterlab & extentions
RUN pip3 install --upgrade --no-cache-dir \
    'jupyterlab~=3.0' \
    jupyterlab_code_formatter \
    jupyterlab_vim \
    jupyterlab-git \
    yapf \
    lckr-jupyterlab-variableinspector \
    && rm -rf ~/.cache/pip 

WORKDIR /tmp/work/