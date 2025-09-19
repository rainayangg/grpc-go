#include <errno.h>
#include <netinet/tcp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>

#include <bcc/bcc_common.h>
#include <bcc/libbpf.h>
#include <bpf/bpf.h>
#include <fcntl.h>

#include "common-koma.h"

#define _GNU_SOURCE

int bpf_init(const char *service_proto) {
  int fd = 0;
  char bpf_prog[50];
  void *mod;

  if (strcmp(service_proto, "memcache-bin") == 0) {
    strcpy(bpf_prog, "memcache-bin_kern.c");
  } else if (strcmp(service_proto, "memcache-id") == 0) {
    strcpy(bpf_prog, "memcache-id_kern.c");
  } else {
    printf("Unknown protocol: %s\n", service_proto);
    exit(1);
  }

  mod = bpf_module_create_c(bpf_prog, 0, NULL, 0, 0, NULL);
  if (!mod) {
    perror("Failed to create BPF module");
    exit(1);
  }

  fd = bcc_prog_load(BPF_PROG_TYPE_SK_SKB, "memcached_koma",
                     bpf_function_start(mod, "memcached_koma"),
                     bpf_function_size(mod, "memcached_koma"),
                     bpf_module_license(mod), bpf_module_kern_version(mod), 0,
                     NULL, 0);

  if (fd == -1) {
    perror("Failed to load BPF program");
    exit(1);
  }

  return fd;
}

static void setnonblocking(int fd) {
  int flags;
  flags = fcntl(fd, F_GETFL, 0);
  flags = fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

int koma_init(void) {
  int komafd;
  komafd = socket(AF_KOMA, SOCK_DGRAM, KOMAPROTO_CONNECTED);
  if (komafd == -1)
    perror("koma Failure: socket(AF_KCM)");
  setnonblocking(komafd);
  return komafd;
}

int koma_attach(int komafd, int csock, int bpf_prog_fd) {
  int error;
  struct kcm_attach attach_info;

  memset(&attach_info, 0, sizeof(attach_info));
  attach_info.fd = csock;
  attach_info.bpf_fd = bpf_prog_fd;

  error = ioctl(komafd, SIOCKOMAATTACH, &attach_info);

  if (error == -1)
    perror("IOCTL ERROR: ioctl(SIOCKOMAATTACH)");
  return error;
}

int koma_pull(int komafd) {
  int error;
  error = ioctl(komafd, SIOCKOMAPULL);
  if (error == -1)
    perror("IOCTL ERROR: ioctl(SIOCKOMAPULL)");
  return error;
}
