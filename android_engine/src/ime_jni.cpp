#include "ime_jni.h"

#include <android/log.h>
#include <android/native_activity.h>
#include <jni.h>

#define ILOGE(...) __android_log_print(ANDROID_LOG_ERROR, "poem-ime", __VA_ARGS__)

namespace poem {

namespace {

// AttachedEnv attaches the current thread to the JVM for the duration of one
// call. Detaching an already-attached thread would break the caller, so it
// only detaches when this call did the attaching.
class AttachedEnv {
  public:
    explicit AttachedEnv(ANativeActivity* activity) : vm_(activity->vm) {
        if (vm_->GetEnv(reinterpret_cast<void**>(&env_), JNI_VERSION_1_6) == JNI_EDETACHED) {
            if (vm_->AttachCurrentThread(&env_, nullptr) == JNI_OK) {
                attached_ = true;
            } else {
                env_ = nullptr;
            }
        }
    }
    ~AttachedEnv() {
        if (attached_) vm_->DetachCurrentThread();
    }
    JNIEnv* get() const { return env_; }

  private:
    JavaVM* vm_;
    JNIEnv* env_ = nullptr;
    bool attached_ = false;
};

bool ClearException(JNIEnv* env) {
    if (env->ExceptionCheck()) {
        env->ExceptionClear();
        return true;
    }
    return false;
}

} // namespace

void SetSoftKeyboardVisible(ANativeActivity* activity, bool visible) {
    AttachedEnv scoped(activity);
    JNIEnv* env = scoped.get();
    if (!env) {
        ILOGE("no JNIEnv for keyboard toggle");
        return;
    }

    jobject activityObj = activity->clazz;
    jclass activityClass = env->GetObjectClass(activityObj);

    // imm = activity.getSystemService(Context.INPUT_METHOD_SERVICE)
    jclass contextClass = env->FindClass("android/content/Context");
    jfieldID serviceField = env->GetStaticFieldID(contextClass, "INPUT_METHOD_SERVICE", "Ljava/lang/String;");
    jobject serviceName = env->GetStaticObjectField(contextClass, serviceField);
    jmethodID getSystemService =
        env->GetMethodID(activityClass, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");
    jobject imm = env->CallObjectMethod(activityObj, getSystemService, serviceName);
    if (ClearException(env) || !imm) {
        ILOGE("InputMethodManager unavailable");
        return;
    }

    // decorView = activity.getWindow().getDecorView()
    jmethodID getWindow = env->GetMethodID(activityClass, "getWindow", "()Landroid/view/Window;");
    jobject window = env->CallObjectMethod(activityObj, getWindow);
    jclass windowClass = env->GetObjectClass(window);
    jmethodID getDecorView = env->GetMethodID(windowClass, "getDecorView", "()Landroid/view/View;");
    jobject decorView = env->CallObjectMethod(window, getDecorView);

    jclass immClass = env->GetObjectClass(imm);
    if (visible) {
        // SHOW_FORCED: the decor view is not an editable widget, so the
        // implicit variant refuses; forced is the established NativeActivity
        // pattern.
        jmethodID showSoftInput = env->GetMethodID(immClass, "showSoftInput", "(Landroid/view/View;I)Z");
        env->CallBooleanMethod(imm, showSoftInput, decorView, 2 /* SHOW_FORCED */);
    } else {
        jclass viewClass = env->GetObjectClass(decorView);
        jmethodID getWindowToken = env->GetMethodID(viewClass, "getWindowToken", "()Landroid/os/IBinder;");
        jobject token = env->CallObjectMethod(decorView, getWindowToken);
        jmethodID hideSoftInput =
            env->GetMethodID(immClass, "hideSoftInputFromWindow", "(Landroid/os/IBinder;I)Z");
        env->CallBooleanMethod(imm, hideSoftInput, token, 0);
    }
    ClearException(env);
}

int UnicodeCharForKey(ANativeActivity* activity, int keyCode, int metaState) {
    AttachedEnv scoped(activity);
    JNIEnv* env = scoped.get();
    if (!env) return 0;

    jclass keyEventClass = env->FindClass("android/view/KeyEvent");
    jmethodID ctor = env->GetMethodID(keyEventClass, "<init>", "(II)V");
    jobject keyEvent = env->NewObject(keyEventClass, ctor, 0 /* ACTION_DOWN */, keyCode);
    if (ClearException(env) || !keyEvent) return 0;
    jmethodID getUnicodeChar = env->GetMethodID(keyEventClass, "getUnicodeChar", "(I)I");
    const int codepoint = env->CallIntMethod(keyEvent, getUnicodeChar, metaState);
    return ClearException(env) ? 0 : codepoint;
}

} // namespace poem
