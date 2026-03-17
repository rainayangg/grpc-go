#include <errno.h>
#include <netinet/tcp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>

#include <fcntl.h>

#include "common-koma.h"

#define _GNU_SOURCE

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

int koma_attach(int komafd, int csock, int initial_conn_window) {
  int error;
  struct koma_attach attach_info;

  memset(&attach_info, 0, sizeof(attach_info));
  attach_info.fd = csock;
  attach_info.bpf_fd = KOMA_ATTACH_EXTENDED_BPF_FD;
  attach_info.initial_conn_window = initial_conn_window;

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
