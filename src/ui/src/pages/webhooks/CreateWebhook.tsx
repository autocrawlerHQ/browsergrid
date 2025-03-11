import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import {$api} from '@/lib/api-client'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ArrowLeft, Loader2, Plus, Trash } from 'lucide-react'

// Form validation schema using Zod
const webhookFormSchema = z.object({
  name: z.string().min(3, 'Name must be at least 3 characters'),
  description: z.string().optional(),
  session_id: z.string().uuid('Must be a valid session ID'),
  webhook_url: z.string().url('Must be a valid URL'),
  event_pattern: z.object({
    method: z.string().min(1, 'Method is required'),
    param_filters: z.record(z.string()).optional(),
  }),
  timing: z.enum(['pre_event', 'post_event']),
  webhook_headers: z.record(z.string()).optional(),
  timeout_seconds: z.number().min(1).max(60).default(10),
  max_retries: z.number().min(0).max(10).default(3),
  active: z.boolean().default(true),
})

type WebhookFormValues = z.infer<typeof webhookFormSchema>

export default function CreateWebhook() {
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState('basic')
  const [headers, setHeaders] = useState<{ key: string; value: string }[]>([
    { key: '', value: '' },
  ])
  const [paramFilters, setParamFilters] = useState<{ key: string; value: string }[]>([
    { key: '', value: '' },
  ])

  // Default form values
  const defaultValues: Partial<WebhookFormValues> = {
    name: '',
    description: '',
    webhook_url: 'https://',
    event_pattern: {
      method: '',
      param_filters: {},
    },
    timing: 'post_event',
    timeout_seconds: 10,
    max_retries: 3,
    active: true,
  }

  // Create form with validation
  const form = useForm<WebhookFormValues>({
    resolver: zodResolver(webhookFormSchema),
    defaultValues,
  })

  // Fetch sessions for dropdown
  const { data: sessions } = useQuery({
    ...($api.get('/api/v1/sessions/', {
      query: {
        status: 'running',
      }
    })),
    queryKey: ['sessions', 'running'],
  })

  // Fetch webhook templates
  const { data: webhookTemplates } = useQuery({
    ...($api.get('/api/v1/webhooks/templates')),
    queryKey: ['webhook-templates'],
  })

  // Create webhook mutation
  const mutation = useMutation({
    mutationFn: (data: WebhookFormValues) => {
      // Process headers from state to data format
      const processedHeaders = headers.reduce((acc, { key, value }) => {
        if (key && value) {
          acc[key] = value
        }
        return acc
      }, {} as Record<string, string>)

      // Process param filters from state to data format
      const processedParamFilters = paramFilters.reduce((acc, { key, value }) => {
        if (key && value) {
          try {
            // Try to parse as JSON if possible
            acc[key] = JSON.parse(value)
          } catch {
            // Otherwise use as string
            acc[key] = value
          }
        }
        return acc
      }, {} as Record<string, any>)

      // Combine processed data
      const processedData = {
        ...data,
        webhook_headers: processedHeaders,
        event_pattern: {
          ...data.event_pattern,
          param_filters: processedParamFilters,
        },
      }

      return $api.post('/api/v1/webhooks/')(processedData)
    },
    onSuccess: (response) => {
      toast.success('Webhook created successfully')
      navigate(`/webhooks/${response.data.id}`)
    },
    onError: (error) => {
      toast.error('Failed to create webhook', {
        description: error.message || 'Please try again',
      })
    },
  })

  // Form submission handler
  function onSubmit(data: WebhookFormValues) {
    mutation.mutate(data)
  }

  // Apply template
  function applyTemplate(templateId: string) {
    if (!webhookTemplates) return

    const template = webhookTemplates.find((tpl) => tpl.id === templateId)
    
    if (template) {
      form.setValue('name', template.name)
      form.setValue('description', template.description || '')
      form.setValue('event_pattern.method', template.event_pattern.method)
      form.setValue('timing', template.timing)
      form.setValue('webhook_url', template.webhook_url)
      
      // Set param filters
      if (template.event_pattern.param_filters) {
        const paramFiltersArray = Object.entries(template.event_pattern.param_filters).map(
          ([key, value]) => ({
            key,
            value: typeof value === 'string' ? value : JSON.stringify(value),
          })
        )
        if (paramFiltersArray.length > 0) {
          setParamFilters(paramFiltersArray)
        }
      }

      // Set headers
      if (template.webhook_headers) {
        const headersArray = Object.entries(template.webhook_headers).map(
          ([key, value]) => ({ key, value })
        )
        if (headersArray.length > 0) {
          setHeaders(headersArray)
        }
      }

      toast.success('Template applied')
    }
  }

  // Handle adding/removing headers
  const addHeader = () => {
    setHeaders([...headers, { key: '', value: '' }])
  }

  const removeHeader = (index: number) => {
    setHeaders(headers.filter((_, i) => i !== index))
  }

  const updateHeader = (index: number, field: 'key' | 'value', value: string) => {
    const newHeaders = [...headers]
    newHeaders[index][field] = value
    setHeaders(newHeaders)
  }

  // Handle adding/removing param filters
  const addParamFilter = () => {
    setParamFilters([...paramFilters, { key: '', value: '' }])
  }

  const removeParamFilter = (index: number) => {
    setParamFilters(paramFilters.filter((_, i) => i !== index))
  }

  const updateParamFilter = (index: number, field: 'key' | 'value', value: string) => {
    const newFilters = [...paramFilters]
    newFilters[index][field] = value
    setParamFilters(newFilters)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button
          variant="outline"
          size="icon"
          onClick={() => navigate('/webhooks')}
          className="h-8 w-8"
        >
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <h1 className="text-3xl font-bold tracking-tight">Create Webhook</h1>
      </div>

      {/* Template selection */}
      {webhookTemplates && webhookTemplates.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Start from Template</CardTitle>
            <CardDescription>
              Choose a pre-configured webhook template to get started quickly
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Select onValueChange={applyTemplate}>
              <SelectTrigger>
                <SelectValue placeholder="Select a template" />
              </SelectTrigger>
              <SelectContent>
                {webhookTemplates.map((template) => (
                  <SelectItem key={template.id} value={template.id}>
                    {template.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </CardContent>
        </Card>
      )}

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="mb-4">
              <TabsTrigger value="basic">Basic Information</TabsTrigger>
              <TabsTrigger value="event">Event Configuration</TabsTrigger>
              <TabsTrigger value="advanced">Advanced Settings</TabsTrigger>
            </TabsList>

            {/* Basic Information */}
            <TabsContent value="basic" className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle>Webhook Details</CardTitle>
                  <CardDescription>
                    Basic information about your webhook
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <FormField
                    control={form.control}
                    name="name"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Name</FormLabel>
                        <FormControl>
                          <Input placeholder="My Webhook" {...field} />
                        </FormControl>
                        <FormDescription>
                          A descriptive name for your webhook
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="description"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Description</FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder="Description of what this webhook does"
                            {...field}
                            value={field.value || ''}
                          />
                        </FormControl>
                        <FormDescription>
                          Optional description of the webhook's purpose
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="session_id"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Browser Session</FormLabel>
                        <Select
                          onValueChange={field.onChange}
                          value={field.value}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="Select a session" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            {sessions?.map((session) => (
                              <SelectItem key={session.id} value={session.id}>
                                {session.id.substring(0, 8)} - {session.browser} {session.browser_version}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          The browser session this webhook will monitor
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="webhook_url"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Webhook URL</FormLabel>
                        <FormControl>
                          <Input placeholder="https://your-server.com/webhook" {...field} />
                        </FormControl>
                        <FormDescription>
                          The URL that will receive webhook events
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>
            </TabsContent>

            {/* Event Configuration */}
            <TabsContent value="event" className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle>Event Pattern</CardTitle>
                  <CardDescription>
                    Configure which events trigger this webhook
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <FormField
                    control={form.control}
                    name="event_pattern.method"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>CDP Method</FormLabel>
                        <FormControl>
                          <Input placeholder="Network.responseReceived" {...field} />
                        </FormControl>
                        <FormDescription>
                          The CDP event method to listen for (e.g., Network.responseReceived)
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <Separator className="my-4" />

                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <FormLabel>Parameter Filters</FormLabel>
                      <Button 
                        type="button" 
                        variant="outline" 
                        size="sm" 
                        onClick={addParamFilter}
                      >
                        <Plus className="h-4 w-4 mr-2" />
                        Add Filter
                      </Button>
                    </div>
                    <FormDescription>
                      Only trigger the webhook when these parameters match
                    </FormDescription>

                    {paramFilters.map((filter, index) => (
                      <div key={index} className="flex items-start gap-2">
                        <Input
                          placeholder="Parameter"
                          value={filter.key}
                          onChange={(e) => updateParamFilter(index, 'key', e.target.value)}
                          className="flex-1"
                        />
                        <Input
                          placeholder="Value"
                          value={filter.value}
                          onChange={(e) => updateParamFilter(index, 'value', e.target.value)}
                          className="flex-1"
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          onClick={() => removeParamFilter(index)}
                          className="shrink-0"
                        >
                          <Trash className="h-4 w-4" />
                        </Button>
                      </div>
                    ))}
                  </div>

                  <Separator className="my-4" />

                  <FormField
                    control={form.control}
                    name="timing"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Timing</FormLabel>
                        <Select
                          onValueChange={field.onChange}
                          defaultValue={field.value}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="Select timing" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value="pre_event">Before Event</SelectItem>
                            <SelectItem value="post_event">After Event</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          When to trigger the webhook relative to the event
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>
            </TabsContent>

            {/* Advanced Settings */}
            <TabsContent value="advanced" className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle>Headers</CardTitle>
                  <CardDescription>
                    Custom HTTP headers to send with webhook requests
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <FormLabel>Request Headers</FormLabel>
                      <Button 
                        type="button" 
                        variant="outline" 
                        size="sm" 
                        onClick={addHeader}
                      >
                        <Plus className="h-4 w-4 mr-2" />
                        Add Header
                      </Button>
                    </div>
                    <FormDescription>
                      Custom HTTP headers to send with webhook requests
                    </FormDescription>

                    {headers.map((header, index) => (
                      <div key={index} className="flex items-start gap-2">
                        <Input
                          placeholder="Header Name"
                          value={header.key}
                          onChange={(e) => updateHeader(index, 'key', e.target.value)}
                          className="flex-1"
                        />
                        <Input
                          placeholder="Value"
                          value={header.value}
                          onChange={(e) => updateHeader(index, 'value', e.target.value)}
                          className="flex-1"
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          onClick={() => removeHeader(index)}
                          className="shrink-0"
                        >
                          <Trash className="h-4 w-4" />
                        </Button>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Delivery Settings</CardTitle>
                  <CardDescription>
                    Configure webhook delivery behavior
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                    <FormField
                      control={form.control}
                      name="timeout_seconds"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Timeout (seconds)</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              {...field}
                              onChange={(e) => field.onChange(Number(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>
                            How long to wait for responses (1-60 seconds)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="max_retries"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Max Retries</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              {...field}
                              onChange={(e) => field.onChange(Number(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>
                            Maximum retry attempts for failed deliveries (0-10)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <FormField
                    control={form.control}
                    name="active"
                    render={({ field }) => (
                      <FormItem className="flex flex-row items-center justify-between">
                        <div className="space-y-0.5">
                          <FormLabel>Active</FormLabel>
                          <FormDescription>
                            Enable or disable this webhook
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>

          <div className="flex justify-end gap-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => navigate('/webhooks')}
            >
              Cancel
            </Button>
            <Button 
              type="submit" 
              disabled={mutation.isPending}
            >
              {mutation.isPending && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              Create Webhook
            </Button>
          </div>
        </form>
      </Form>
    </div>
  )
} 