ARG ARCH=arm64
ARG hbn_nvcr_tag

# use HBN image as base. for DPF MVP same custom image will be used for hbn main and init containers.
# using 2/6 as a base image for now, due to bug https://redmine.mellanox.com/issues/3821408
# use 2.7 later?

FROM --platform=linux/${ARCH} nvcr.io/nvidia/doca/doca_hbn:${hbn_nvcr_tag}

# Install necessary packages
RUN apt-get update && \
    apt-get install -y \
        jq


WORKDIR /
COPY files/initbootstrap.sh .
COPY files/hbn_files /hbn_files

