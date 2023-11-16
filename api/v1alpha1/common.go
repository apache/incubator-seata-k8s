package v1alpha1

import apiv1 "k8s.io/api/core/v1"

// +k8s:deepcopy-gen=true

type CRDCommonFields struct {
	// +kubebuilder:validation:Optional
	// +nullable
	Replicas *int32 `json:"replicas,omitempty"`
	// +kubebuilder:validation:Optional
	SelectorLabels map[string]string `json:"selectorLabels"`
	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector"`
	// +kubebuilder:validation:Optional
	// +nullable
	Affinity *apiv1.Affinity `json:"affinity"`
	// +kubebuilder:validation:Optional
	Toleration []apiv1.Toleration `json:"toleration"`
	// +kubebuilder:validation:Optional
	// +nullable
	Service *Service `json:"service"`
	// +kubebuilder:validation:Optional
	RBAC *RBAC `json:"rbac"`
	// +kubebuilder:validation:Optional
	ServiceAccount *ServiceAccount `json:"serviceAccount"`
	// +kubebuilder:validation:Optional
	// +nullable
	AutoScaling *AutoScaling `json:"autoScaling"`
	// +kubebuilder:validation:Optional
	// +nullable
	PodSecurityContext *apiv1.PodSecurityContext `json:"podSecurityContext"`

	// +kubebuilder:validation:Optional
	EnableStatus bool `json:"enableStatus"`
	// +kubebuilder:validation:Optional
	EnableHigressIstio bool `json:"enableHigressIstio"`
	// +kubebuilder:validation:Optional
	EnableIstioAPI bool `json:"enableIstioAPI"`
	// +kubebuilder:validation:Optional
	IstioNamespace string `json:"istioNamespace"`
	// +kubebuilder:validation:Optional
	Revision string `json:"revision"`
	// +kubebuilder:validation:Optional
	// +nullable
	Istiod *Istio `json:"istiod"`
	// +kubebuilder:validation:Optional
	// +nullable
	MultiCluster *MultiCluster `json:"multiCluster"`
	Local        bool          `json:"local"`
	// +kubebuilder:validation:Enum=third-party-jwt;first-party-jwt
	JwtPolicy string `json:"jwtPolicy"`
}

// +k8s:deepcopy-gen=true

type ContainerCommonFields struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name"`
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations"`
	Image       Image             `json:"image"`
	// +kubebuilder:validation:Optional
	ImagePullSecrets []apiv1.LocalObjectReference `json:"imagePullSecrets"`
	// +kubebuilder:validation:Optional
	Env map[string]string `json:"env"`
	// +kubebuilder:validation:Optional
	ReadinessProbe *apiv1.Probe `json:"readinessProbe"`
	// +kubebuilder:validation:Optional
	Ports []apiv1.ContainerPort `json:"ports"`
	// +kubebuilder:validation:Optional
	Resources *apiv1.ResourceRequirements `json:"resources"`
	// +kubebuilder:validation:Optional
	SecurityContext *apiv1.SecurityContext `json:"securityContext"`
	// +kubebuilder:validation:Optional
	LogLevel string `json:"logLevel"`
	// +kubebuilder:validation:Optional
	LogAsJson bool `json:"logAsJson"`
}

// +k8s:deepcopy-gen=true

type Image struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	// +kubebuilder:validation:Enum="";Always;Never;IfNotPresent
	ImagePullPolicy apiv1.PullPolicy `json:"imagePullPolicy"`
}

// +k8s:deepcopy-gen=true

type ServiceAccount struct {
	Enable bool `json:"enable"`
	// +kubebuilder:validation:Optional
	Name string `json:"name"`
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations"`
}

// +k8s:deepcopy-gen=true

type AutoScaling struct {
	Enable                         bool   `json:"enable"`
	MinReplicas                    *int32 `json:"minReplicas"`
	MaxReplicas                    int32  `json:"maxReplicas"`
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage"`
}

// +k8s:deepcopy-gen=true

type RBAC struct {
	Enable bool `json:"enable"`
}

// +k8s:deepcopy-gen=true

type Istio struct {
	EnableAnalysis bool `json:"enableAnalysis"`
}

// +k8s:deepcopy-gen=true

type MultiCluster struct {
	Enable      bool   `json:"enable"`
	ClusterName string `json:"clusterName"`
}

// +k8s:deepcopy-gen=true

type Service struct {
	Type  string              `json:"type"`
	Ports []apiv1.ServicePort `json:"ports"`
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations"`
	// +kubebuilder:validation:Optional
	LoadBalancerIP string `json:"loadBalancerIP"`
	// +kubebuilder:validation:Optional
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges"`
	// +kubebuilder:validation:Optional
	ExternalTrafficPolicy string `json:"externalTrafficPolicy"`
}
