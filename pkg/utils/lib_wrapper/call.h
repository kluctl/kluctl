
#ifndef VARIADIC_H
#define VARIADIC_H

#include <sys/types.h>
#include <unistd.h>

void* call_vp_ptrs(void* f, int argc, void **argv);
void* call_vp_ptrs_vargs(void* f, int nptrs, void **ptrs, int argc, void **argv);
void call_v_ptrs(void* f, int argc, void **argv);
int call_i_ptrs(void* f, int argc, void **argv);

void* call_vp_ss(void* f, ssize_t a1);
void* call_vp_vp_ss(void* f, void *a1, ssize_t a2);

#endif
