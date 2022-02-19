/*
Unless explicitly stated otherwise all files in this repository are licensed
under the MIT License.
This product includes software developed at Datadog (https://www.datadoghq.com/).
Copyright 2018 Datadog, Inc.
*/

#ifndef VARIADIC_H
#define VARIADIC_H

#include "Python.h"

PyObject* _go_PyObject_CallFunctionObjArgs(PyObject *callable, int argc, PyObject **args);
PyObject* _go_PyObject_CallMethodObjArgs(PyObject *obj, PyObject *name, int argc, PyObject **args);

#endif