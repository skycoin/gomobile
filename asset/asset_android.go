// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asset

/*
#cgo LDFLAGS: -landroid
#include <android/asset_manager.h>
#include <android/asset_manager_jni.h>
#include <jni.h>
#include <stdlib.h>

static jobject GetAssetManager(JNIEnv *const env, jobject ctx) {
	// Equivalent to:
	//	assetManager = ctx.getResources().getAssets();
	jclass ctx_clazz = (*env)->FindClass(env, "android/content/Context");
	jmethodID getres_id = (*env)->GetMethodID(env, ctx_clazz, "getResources", "()Landroid/content/res/Resources;");
	jobject res = (*env)->CallObjectMethod(env, ctx, getres_id);
	jclass res_clazz = (*env)->FindClass(env, "android/content/res/Resources");
	jmethodID getam_id = (*env)->GetMethodID(env, res_clazz, "getAssets", "()Landroid/content/res/AssetManager;");
	jobject am = (*env)->CallObjectMethod(env, res, getam_id);

	return am;
}

static AAssetManager* asset_manager_init(uintptr_t java_vm, uintptr_t jni_env, jobject ctx) {
	JavaVM* vm = (JavaVM*)java_vm;
	JNIEnv* env = (JNIEnv*)jni_env;

	jobject am = GetAssetManager(env, ctx);

	// Pin the AssetManager and load an AAssetManager from it.
	am = (*env)->NewGlobalRef(env, am);
	return AAssetManager_fromJava(env, am);
}

static jsize GetArrayLength(uintptr_t jni_env, jobjectArray array) {
	JNIEnv* env = (JNIEnv*)jni_env;
	return (*env)->GetArrayLength(env, array);
}

static jobject GetObjectArrayElement(uintptr_t jni_env, jobjectArray array, jsize index) {
	JNIEnv* env = (JNIEnv*)jni_env;
	return (*env)->GetObjectArrayElement(env, array, index);
}

static jobjectArray AssetManager_list(uintptr_t jni_env, jobject ctx, char* path) {
	JNIEnv* env = (JNIEnv*)jni_env;

	jobject am = GetAssetManager(env, ctx);
	jclass amClass = (*env)->FindClass(env, "android/content/res/AssetManager");
	jmethodID listId = (*env)->GetMethodID(env, amClass, "list", "(Ljava/lang/String;)[Ljava/lang/String;");
	jstring jpath = (*env)->NewStringUTF(env, path);
	return  (*env)->CallObjectMethod(env, am, listId, jpath);
}

static void ReleaseAssetList(char** const ppSz) {
	if (ppSz != NULL) {
		int i = 0;
		while (ppSz[i] != NULL) {
			free(ppSz[i]);
		}
		free(ppSz);
	}
}

static jstring GetFilesDir(uintptr_t java_vm, uintptr_t jni_env, jobject ctx) {
	JNIEnv* env = (JNIEnv*)jni_env;

	jclass context = (*env)->FindClass(env, "android/content/Context");
	if(context == NULL) {
		return NULL;
	}
	jmethodID getFilesDir = (*env)->GetMethodID(env, context, "getFilesDir", "()Ljava/io/File;");
	if(getFilesDir == NULL){
		return NULL;
	}

	jobject f = (*env)->CallObjectMethod(env, ctx, getFilesDir);
	if (f == NULL) {
		return NULL;
	}

	jclass file = (*env)->FindClass(env, "java/io/File");
	if (file == NULL) {
		return NULL;
	}

	jmethodID getAbsolutePath = (*env)->GetMethodID(env, file, "getAbsolutePath", "()Ljava/lang/String;");
	if (getAbsolutePath == NULL) {
		return NULL;
	}

	jstring path = (jstring)(*env)->CallObjectMethod(env, f, getAbsolutePath);
	return path;
}

static char const* GetStringUTFChars(uintptr_t jni_env, jstring str) {
	JNIEnv* env = (JNIEnv*)jni_env;
	return (*env)->GetStringUTFChars(env, str, 0);
}

static void ReleaseStringUTFChars(uintptr_t jni_env, jstring str, char* csz) {
	JNIEnv* env = (JNIEnv*)jni_env;
	(*env)->ReleaseStringUTFChars(env, str, csz);
}



*/
import "C"
import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"unsafe"

	"github.com/SkycoinProject/gomobile/internal/mobileinit"
)

var assetOnce sync.Once

// asset_manager is the asset manager of the app.
var assetManager *C.AAssetManager

func assetInit() {
	err := mobileinit.RunOnJVM(func(vm, env, ctx uintptr) error {
		assetManager = C.asset_manager_init(C.uintptr_t(vm), C.uintptr_t(env), C.jobject(ctx))
		return nil
	})
	if err != nil {
		log.Fatalf("asset: %v", err)
	}
}

func openAsset(name string) (File, error) {
	assetOnce.Do(assetInit)
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	a := &asset{
		ptr:  C.AAssetManager_open(assetManager, cname, C.AASSET_MODE_UNKNOWN),
		name: name,
	}
	if a.ptr == nil {
		return nil, a.errorf("open", "bad asset")
	}
	return a, nil
}

func GetAssetList(path string) []string {
	var gassetList []string
	err := mobileinit.RunOnJVM(func(vm, env, ctx uintptr) error {
		//fmt.Printf("getAssetList : path '%s'\n", path)
		cpath := C.CString(path)
		defer C.free(unsafe.Pointer(cpath))
		jassetList := C.AssetManager_list(C.uintptr_t(env), C.jobject(ctx), cpath)
		assetCount := C.GetArrayLength(C.uintptr_t(env), jassetList)
		//fmt.Printf("assetCount '%d'\n", assetCount)
		if assetCount > 0 {
			gassetList = make([]string, assetCount)
			for i := C.jsize(0); i < assetCount; i++ {
				jasset := C.jstring(C.GetObjectArrayElement(C.uintptr_t(env), jassetList, i))
				casset := C.GetStringUTFChars(C.uintptr_t(env), jasset)
				gasset := C.GoString(casset)
				C.ReleaseStringUTFChars(C.uintptr_t(env), jasset, casset)
				gassetList[i] = gasset
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("failed to getAssetList %v", err)
	}
	return gassetList
}

func GetFilesDir() string {
	var gpath string
	err := mobileinit.RunOnJVM(func(vm, env, ctx uintptr) error {
		jpath := C.GetFilesDir(C.uintptr_t(vm), C.uintptr_t(env), C.jobject(ctx))
		cpath := C.GetStringUTFChars(C.uintptr_t(env), jpath)
		gpath = C.GoString(cpath)
		C.ReleaseStringUTFChars(C.uintptr_t(env), jpath, cpath)
		return nil
	})
	if err != nil {
		log.Fatalf("failed to getFilesDir %v", err)
	}
	return gpath
}

type asset struct {
	ptr  *C.AAsset
	name string
}

func (a *asset) errorf(op string, format string, v ...interface{}) error {
	return &os.PathError{
		Op:   op,
		Path: a.name,
		Err:  fmt.Errorf(format, v...),
	}
}

func (a *asset) Read(p []byte) (n int, err error) {
	n = int(C.AAsset_read(a.ptr, unsafe.Pointer(&p[0]), C.size_t(len(p))))
	if n == 0 && len(p) > 0 {
		return 0, io.EOF
	}
	if n < 0 {
		return 0, a.errorf("read", "negative bytes: %d", n)
	}
	return n, nil
}

func (a *asset) Seek(offset int64, whence int) (int64, error) {
	// TODO(crawshaw): use AAsset_seek64 if it is available.
	off := C.AAsset_seek(a.ptr, C.off_t(offset), C.int(whence))
	if off == -1 {
		return 0, a.errorf("seek", "bad result for offset=%d, whence=%d", offset, whence)
	}
	return int64(off), nil
}

func (a *asset) Close() error {
	C.AAsset_close(a.ptr)
	return nil
}
