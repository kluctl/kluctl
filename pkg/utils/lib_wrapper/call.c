
#include "call.h"

void* call_vp_ptrs(void* f, int argc, void **argv) {
    switch (argc) {
        case 0: {
            typedef void* (*F)();
            return ((F)(f))();
        }
        case 1: {
            typedef void* (*F)(void*);
            return ((F)(f))(argv[0]);
        }
        case 2: {
            typedef void* (*F)(void*,void*);
            return ((F)(f))(argv[0],argv[1]);
        }
        case 3: {
            typedef void* (*F)(void*,void*,void*);
            return ((F)(f))(argv[0],argv[1],argv[2]);
        }
        case 4: {
            typedef void* (*F)(void*,void*,void*,void*);
            return ((F)(f))(argv[0],argv[1],argv[2],argv[3]);
        }
    }
    return NULL;
}

void* call_vp_ptrs_vargs(void* f, int nptrs, void **ptrs, int argc, void **argv) {
    switch (nptrs) {
        case 0: {
            // crash!!
            return &*((int*)NULL);
        }
        case 1: {
            typedef void* (*F)(void*, ...);
            switch (argc) {
            case 0: return ((F)(f))(ptrs[0]);
            case 1: return ((F)(f))(ptrs[0], argv[0]);
            case 2: return ((F)(f))(ptrs[0], argv[0], argv[1]);
            case 3: return ((F)(f))(ptrs[0], argv[0], argv[1], argv[2]);
            case 4: return ((F)(f))(ptrs[0], argv[0], argv[1], argv[2], argv[3]);
            }
        }
        case 2: {
            typedef void* (*F)(void*,void*, ...);
            switch (argc) {
            case 0: return ((F)(f))(ptrs[0], ptrs[1]);
            case 1: return ((F)(f))(ptrs[0], ptrs[1], argv[0]);
            case 2: return ((F)(f))(ptrs[0], ptrs[1], argv[0], argv[1]);
            case 3: return ((F)(f))(ptrs[0], ptrs[1], argv[0], argv[1], argv[2]);
            case 4: return ((F)(f))(ptrs[0], ptrs[1], argv[0], argv[1], argv[2], argv[3]);
            }
        }
    }
    // crash!!
    return &*((int*)NULL);
}

int call_i_ptrs(void* f, int argc, void **argv) {
    switch (argc) {
        case 0: {
            typedef int (*F)();
            return ((F)(f))();
        }
        case 1: {
            typedef int (*F)(void*);
            return ((F)(f))(argv[0]);
        }
        case 2: {
            typedef int (*F)(void*,void*);
            return ((F)(f))(argv[0],argv[1]);
        }
        case 3: {
            typedef int (*F)(void*,void*,void*);
            return ((F)(f))(argv[0],argv[1],argv[2]);
        }
        case 4: {
            typedef int (*F)(void*,void*,void*,void*);
            return ((F)(f))(argv[0],argv[1],argv[2],argv[3]);
        }
    }
    return 0;
}

void call_v_ptrs(void* f, int argc, void **argv) {
    call_vp_ptrs(f, argc, argv);
}

void* call_vp_ss(void* f, ssize_t i) {
    typedef void* (*F)(ssize_t);
    return ((F)(f))(i);
}

void* call_vp_vp_ss(void* f, void *a1, ssize_t a2) {
    typedef void* (*F)(void*,ssize_t);
    return ((F)(f))(a1,a2);
}
