package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type SecretManager struct {
	client kubernetes.Interface
}

func NewSecretManager(client kubernetes.Interface) *SecretManager {
	return &SecretManager{client: client}
}

// GetKubeClient obtiene un cliente de Kubernetes autenticado ya sea dentro del clúster o mediante kubeconfig local
func GetKubeClient() (kubernetes.Interface, error) {
	// 1. Intentar configuración dentro del clúster (InClusterConfig)
	config, err := rest.InClusterConfig()
	if err == nil {
		return kubernetes.NewForConfig(config)
	}

	// 2. Intentar archivo KUBECONFIG desde variable de entorno
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if kubeconfigEnv != "" {
		if cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigEnv); err == nil {
			return kubernetes.NewForConfig(cfg)
		}
	}

	// 3. Intentar ruta por defecto en home (~/.kube/config)
	if home := homedir.HomeDir(); home != "" {
		kubeconfigPath := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(kubeconfigPath); err == nil {
			if cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath); err == nil {
				return kubernetes.NewForConfig(cfg)
			}
		}
	}

	return nil, fmt.Errorf("no se pudo inicializar la configuración de Kubernetes (in-cluster o ~/.kube/config no disponibles): %w", err)
}

// GetExistingDBPassword consulta el Secret <proyecto>-db-secrets para recuperar la contraseña existente
func (sm *SecretManager) GetExistingDBPassword(ctx context.Context, namespace, projectName string) (string, error) {
	secretName := fmt.Sprintf("%s-db-secrets", projectName)
	sec, err := sm.client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("error al consultar secret %s en namespace %s: %w", secretName, namespace, err)
	}

	if passBytes, ok := sec.Data["DB_PASSWORD"]; ok && len(passBytes) > 0 {
		return string(passBytes), nil
	}
	if passStr, ok := sec.StringData["DB_PASSWORD"]; ok && len(passStr) > 0 {
		return passStr, nil
	}
	return "", nil
}

// EnsureDBSecret crea o actualiza en memoria el Secret con las credenciales de base de datos
func (sm *SecretManager) EnsureDBSecret(ctx context.Context, namespace, projectName string, res *DBProvisionResult) error {
	secretName := fmt.Sprintf("%s-db-secrets", projectName)

	stringData := map[string]string{
		"DB_HOST":      res.Host,
		"DB_PORT":      strconv.Itoa(res.Port),
		"DB_NAME":      res.Database,
		"DB_USER":      res.Username,
		"DB_PASSWORD":  res.Password,
		"DATABASE_URL": res.DatabaseURL,
	}

	sec, err := sm.client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "sammcore-deployer",
						"project":                      projectName,
					},
				},
				Type:       corev1.SecretTypeOpaque,
				StringData: stringData,
			}
			_, err := sm.client.CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("error al crear secret %s: %w", secretName, err)
			}
			return nil
		}
		return fmt.Errorf("error al verificar secret %s: %w", secretName, err)
	}

	sec.StringData = stringData
	_, err = sm.client.CoreV1().Secrets(namespace).Update(ctx, sec, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error al actualizar secret %s: %w", secretName, err)
	}
	return nil
}

// DeleteDBSecret elimina el secret de base de datos si existe
func (sm *SecretManager) DeleteDBSecret(ctx context.Context, namespace, projectName string) error {
	secretName := fmt.Sprintf("%s-db-secrets", projectName)
	err := sm.client.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error al eliminar secret %s: %w", secretName, err)
	}
	return nil
}

// EnsureRegistrySecret copia sammcore-registry-secret al namespace del proyecto si existe en el namespace deployer
func (sm *SecretManager) EnsureRegistrySecret(ctx context.Context, targetNamespace string) error {
	srcSecret, err := sm.client.CoreV1().Secrets("deployer").Get(ctx, "sammcore-registry-secret", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // No es obligatorio si las imágenes son públicas o locales
		}
		return err
	}

	targetSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sammcore-registry-secret",
			Namespace: targetNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sammcore-deployer",
			},
		},
		Type: srcSecret.Type,
		Data: srcSecret.Data,
	}

	_, err = sm.client.CoreV1().Secrets(targetNamespace).Create(ctx, targetSecret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("error asegurando registry secret en %s: %w", targetNamespace, err)
	}
	return nil
}
