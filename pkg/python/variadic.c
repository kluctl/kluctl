/*
Unless explicitly stated otherwise all files in this repository are licensed
under the MIT License.
This product includes software developed at Datadog (https://www.datadoghq.com/).
Copyright 2018 Datadog, Inc.
*/

#include "Python.h"

PyObject* _go_PyObject_CallFunctionObjArgs(PyObject *callable, int argc, PyObject **argv) {

    PyObject *result = NULL;
    switch (argc) {
        case 0:
            return PyObject_CallFunctionObjArgs(callable, NULL);
        case 1:
            return PyObject_CallFunctionObjArgs(callable, argv[0], NULL);
        case 2:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], NULL);
        case 3:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], NULL);
        case 4:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], NULL);
        case 5:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], NULL);
        case 6:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], NULL);
        case 7:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], NULL);
        case 8:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], NULL);
        case 9:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], NULL);
        case 10:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], NULL);
        case 11:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], NULL);
        case 12:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], NULL);
        case 13:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], NULL);
        case 14:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], NULL);
        case 15:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], NULL);
        case 16:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], NULL);
        case 17:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], argv[16], NULL);
        case 18:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], argv[16], argv[17], NULL);
        case 19:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], argv[16], argv[17], argv[18], NULL);
        case 20:
            return PyObject_CallFunctionObjArgs(callable, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], argv[16], argv[17], argv[18], argv[19], NULL);
    }
    return result;
}
PyObject* _go_PyObject_CallMethodObjArgs(PyObject *obj, PyObject *name, int argc, PyObject **argv) {

    PyObject *result = NULL;
    switch (argc) {
        case 0:
            return PyObject_CallMethodObjArgs(obj, name, NULL);
        case 1:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], NULL);
        case 2:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], NULL);
        case 3:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], NULL);
        case 4:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], NULL);
        case 5:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], NULL);
        case 6:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], NULL);
        case 7:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], NULL);
        case 8:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], NULL);
        case 9:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], NULL);
        case 10:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], NULL);
        case 11:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], NULL);
        case 12:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], NULL);
        case 13:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], NULL);
        case 14:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], NULL);
        case 15:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], NULL);
        case 16:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], NULL);
        case 17:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], argv[16], NULL);
        case 18:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], argv[16], argv[17], NULL);
        case 19:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], argv[16], argv[17], argv[18], NULL);
        case 20:
            return PyObject_CallMethodObjArgs(obj, name, argv[0], argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7], argv[8], argv[9], argv[10], argv[11], argv[12], argv[13], argv[14], argv[15], argv[16], argv[17], argv[18], argv[19], NULL);
    }
    return result;
}
