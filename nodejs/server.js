// Function to mutate the StatefulSet based on the admission review
function mutateStatefulSet(admissionReview) {
  // Extract the raw object from the admission review request
  const raw = admissionReview.request.object.raw;

  // Parse the raw object as JSON to get the StatefulSet
  const statefulSet = JSON.parse(raw);
  
  // Extract UID from the admission request
  const uid = admissionReview.request.uid;

  // Add the UID as a label to the StatefulSet
  statefulSet.metadata.labels = {
    ...statefulSet.metadata.labels,
    "uid": uid,
  };

  if (!statefulSet.spec.template.metadata.labels) {
  statefulSet.spec.template.metadata.labels = {};
  }

  // Add the UID as a label to the Pod template
  statefulSet.spec.template.metadata.labels = {
    ...statefulSet.spec.template.metadata.labels,
    "uid": uid,
  }
  

  // Convert the mutated StatefulSet back to base64
  const mutatedRaw = Buffer.from(JSON.stringify(statefulSet)).toString('base64');

  // Create the admission response
  return {
    response: {
      allowed: true,
      uid: admissionReview.request.uid,
      patch: mutatedRaw,
      patchType: 'JSONPatch',
    },
  };
}
