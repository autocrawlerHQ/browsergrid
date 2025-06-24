import axios, { AxiosRequestConfig } from 'axios';

// Create a custom axios instance with base configuration
const axiosInstance = axios.create({
  baseURL: import.meta.env.VITE_BROWSERGRID_API_URL || 'http://localhost:8765',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add request interceptor for authentication if needed
axiosInstance.interceptors.request.use(
  (config) => {
    // Add auth token here if needed
    // const token = localStorage.getItem('token');
    // if (token) {
    config.headers.Authorization = `Bearer 123`;
    // }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Add response interceptor for error handling
axiosInstance.interceptors.response.use(
  (response) => response,
  (error) => {
    // Handle common errors here
    if (error.response?.status === 401) {
      // Handle unauthorized
      console.error('Unauthorized access');
    }
    return Promise.reject(error);
  }
);

// Export the custom instance with the signature orval expects
export const customInstance = <T = any>(config: AxiosRequestConfig): Promise<T> => {
  return axiosInstance(config).then(({ data }) => data);
};

export default customInstance; 