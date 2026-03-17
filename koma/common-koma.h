#ifndef COMMON_KOMA_H
#define COMMON_KOMA_H

#define _GNU_SOURCE

#include <linux/kcm.h>

#ifndef AF_KOMA
/* From linux/socket.h */
#define AF_KOMA 46 /* Kernel Connection Multiplexor*/
#endif

#ifndef KOMAPROTO_CONNECTED
/* From linux/kcm.h */
#define KOMAPROTO_CONNECTED 0
#endif

#ifndef SIOCKOMAATTACH
#define SIOCKOMAATTACH (SIOCPROTOPRIVATE + 0)
#endif

#ifndef SIOCKOMAPULL
#define SIOCKOMAPULL (SIOCPROTOPRIVATE + 3)
#endif

struct koma_attach {
	int fd;
	int bpf_fd;
	int initial_conn_window;
};

int bpf_init(const char *);
int koma_init(void);
int koma_attach(int komafd, int csock, int initial_conn_window);
int koma_pull(int komafd);

#endif /* COMMON_KOMA_H */
