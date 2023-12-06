package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	admission "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/kubernetes/pkg/apis/apps/v1"

	"encoding/json"
)


var (
	// Initialize runtime scheme, codec factory, and deserializer

	/* 
	Creates a new instance of the Kubernetes runtime.Scheme. 
	This scheme is a central registry for kubernetes types and their conversion functions. 
	It is used for encoding and decoding objects in Kubernetes.
	*/
	runtimeScheme = runtime.NewScheme()
	// Instantiate serializer.CodecFactory, which is used for encoding and decoding objects
	codecFactory  = serializer.NewCodecFactory(runtimeScheme)
	/* 
	Codec is a Serializer that deals with the details of versioning objects.
	It offers the same interface as Serializer, so this is a marker to consumers that care 
	about the version of the objects they receive
	*/
	deserializer  = codecFactory.UniversalDeserializer()
)

// This is a special Go function that is automatically called during initialization of the program.
func init() {
	// Adds the corev1 types to runtimeScheme 
	_ = corev1.AddToScheme(runtimeScheme)
	// Adds admission api types to runtimeScheme 
	_ = admission.AddToScheme(runtimeScheme)
	// Adds types from apps/v1 to runtimeScheme
	_ = v1.AddToScheme(runtimeScheme)
}


// Declare new function to handle an admissionReview and produce an admissionResponse
type admitv1Func func(admission.AdmissionReview) *admission.AdmissionResponse

// Declare new structure to hold function admitv1Func
type admitHandler struct {
	v1 admitv1Func
}

// Constructor for admitHandler
func AdmitHandler(f admitv1Func) admitHandler {
	return admitHandler{
		v1: f,
	}
}


// serve handles the http portion of a request prior to handing to an admit function
func serve(w http.ResponseWriter, r *http.Request, admit admitHandler) {
	// This is a byte slice to store the store the content of http request 
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Error().Msgf("contentType=%s, expected application/json", contentType)
		return
	}

	// Print error to logs and return http error if request fails 
	log.Info().Msgf("handling request: %s", body)
	var responseObj runtime.Object
	if obj, gvk, err := deserializer.Decode(body, nil, nil); err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		log.Error().Msg(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return

	// If decoding succeeeds 
	} else {
		// Store admission.AdmissionReview from decoded obj
		requestedAdmissionReview, ok := obj.(*admission.AdmissionReview)
		if !ok {
			log.Error().Msgf("Expected v1.AdmissionReview but got: %T", obj)
			return
		}
		// Create admission review response 
		responseAdmissionReview := &admission.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		// Set response to result of call to admit.v1 function
		responseAdmissionReview.Response = admit.v1(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview

	}
	log.Info().Msgf("sending response: %v", responseObj)
	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		log.Err(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		log.Err(err)
	}
}

// Called when the mutating webhook endpoint /mutate is requested 
func serveMutate(w http.ResponseWriter, r *http.Request) {
	serve(w, r, AdmitHandler(mutate))
}
// Called when the validating webhook endpoint /validate is requested 
func serveValidate(w http.ResponseWriter, r *http.Request) {
	serve(w, r, AdmitHandler(validate))
}

func mutate(ar admission.AdmissionReview) *admission.AdmissionResponse {
    log.Info().Msgf("mutating statefulset")
    statefulsetResource := metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
    if ar.Request.Resource != statefulsetResource {
        log.Error().Msgf("expect resource to be %s", statefulsetResource)
        return nil
    }
    raw := ar.Request.Object.Raw
    statefulSet := appsv1.StatefulSet{}

    if _, _, err := deserializer.Decode(raw, nil, &statefulSet); err != nil {
        log.Err(err)
        return &admission.AdmissionResponse{
            Result: &metav1.Status{
                Message: err.Error(),
            },
        }
    }

    userName := ar.Request.UserInfo.Username
    pt := admission.PatchTypeJSONPatch
    statefulsetPatch := fmt.Sprintf(`[
        { "op": "add", "path": "/spec/template/metadata/labels/userName", "value": "%s" },
        { "op": "add", "path": "/spec/selector/matchLabels/userName", "value": "%s" }
    ]`, userName, userName)

    return &admission.AdmissionResponse{Allowed: true, PatchType: &pt, Patch: []byte(statefulsetPatch)}
}


// verify if a Deployment has the 'prod' prefix name
func validate(ar admission.AdmissionReview) *admission.AdmissionResponse {
	log.Info().Msgf("validating deployments")
	deploymentResource := metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if ar.Request.Resource != deploymentResource {
		log.Error().Msgf("expect resource to be %s", deploymentResource)
		return nil
	}
	raw := ar.Request.Object.Raw
	deployment := appsv1.Deployment{}
	if _, _, err := deserializer.Decode(raw, nil, &deployment); err != nil {
		log.Err(err)
		return &admission.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}
	if !strings.HasPrefix(deployment.GetName(), "prod-") {
		return &admission.AdmissionResponse{
			Allowed: false, Result: &metav1.Status{
				Message: "Statefulset's prefix name \"prod\" not found",
			},
		}
	}
	return &admission.AdmissionResponse{Allowed: true}
}

func main() {
	var tlsKey, tlsCert string
	flag.StringVar(&tlsKey, "tlsKey", "/etc/certs/tls.key", "Path to the TLS key")
	flag.StringVar(&tlsCert, "tlsCert", "/etc/certs/tls.crt", "Path to the TLS certificate")
	flag.Parse()
	http.HandleFunc("/mutate", serveMutate)
	http.HandleFunc("/validate", serveValidate)
	log.Info().Msg("Server started ...")
	log.Fatal().Err(http.ListenAndServeTLS(":8443", tlsCert, tlsKey, nil)).Msg("webhook server exited")
}