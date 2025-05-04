import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { $api } from '@/lib/api-client'
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
  CardFooter,
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
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ArrowLeft, Loader2 } from 'lucide-react'

// Form validation schema using Zod
const sessionFormSchema = z.object({
  browser: z.enum(['chrome', 'firefox', 'edge', 'safari']),
  browser_version: z.enum(['latest', 'stable', 'canary', 'dev']),
  operating_system: z.enum(['windows', 'macos', 'linux']),
  screen: z.object({
    width: z.number().min(800).max(3840).default(1280),
    height: z.number().min(600).max(2160).default(720),
    dpi: z.number().min(72).max(300).default(96),
    scale: z.number().min(1).max(3).default(1),
  }),
  resource_limits: z.object({
    cpu: z.number().min(1).max(8).optional(),
    memory: z.string().optional(),
    timeout_minutes: z.number().min(5).max(240).default(30),
  }),
  headless: z.boolean().default(false),
  record_network: z.boolean().default(true),
  record_console: z.boolean().default(true),
  work_pool_id: z.string().optional(),
})

type SessionFormValues = z.infer<typeof sessionFormSchema>

export default function CreateSession() {
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState('basic')

  // Default form values
  const defaultValues: Partial<SessionFormValues> = {
    browser: 'chrome',
    browser_version: 'latest',
    operating_system: 'linux',
    screen: {
      width: 1280,
      height: 720,
      dpi: 96,
      scale: 1,
    },
    resource_limits: {
      cpu: 2,
      memory: '2048m',
      timeout_minutes: 30,
    },
    headless: false,
    record_network: true,
    record_console: true,
  }

  // Create form with validation
  const form = useForm<SessionFormValues>({
    resolver: zodResolver(sessionFormSchema),
    defaultValues,
  })

  // Fetch work pools for dropdown
  const { data: workPools } = useQuery({
    ...($api.get('/v1/workerpools/pools')),
    queryKey: ['workpools'],
  })

  // Create session mutation
  const mutation = useMutation({
    mutationFn: (data: SessionFormValues) =>
      $api.post('/api/v1/sessions/')(data),
    onSuccess: (response) => {
      toast.success('Browser session created successfully')
      navigate(`/sessions/${response.data.id}`)
    },
    onError: (error) => {
      toast.error('Failed to create session', {
        description: error.message || 'Please try again',
      })
    },
  })

  // Form submission handler
  function onSubmit(data: SessionFormValues) {
    mutation.mutate(data)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button
          variant="outline"
          size="icon"
          onClick={() => navigate(-1)}
          className="h-8 w-8"
        >
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <h1 className="text-3xl font-bold tracking-tight">Create Browser Session</h1>
      </div>

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="mb-4">
              <TabsTrigger value="basic">Basic Configuration</TabsTrigger>
              <TabsTrigger value="advanced">Advanced Settings</TabsTrigger>
              <TabsTrigger value="resources">Resource Limits</TabsTrigger>
            </TabsList>

            {/* Basic Configuration */}
            <TabsContent value="basic" className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle>Browser Settings</CardTitle>
                  <CardDescription>
                    Configure the browser type and version for your session
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <FormField
                      control={form.control}
                      name="browser"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Browser</FormLabel>
                          <Select
                            onValueChange={field.onChange}
                            defaultValue={field.value}
                          >
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue placeholder="Select browser" />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="chrome">Chrome</SelectItem>
                              <SelectItem value="firefox">Firefox</SelectItem>
                              <SelectItem value="edge">Edge</SelectItem>
                              <SelectItem value="safari">Safari</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormDescription>
                            The browser to use for this session
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="browser_version"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Browser Version</FormLabel>
                          <Select
                            onValueChange={field.onChange}
                            defaultValue={field.value}
                          >
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue placeholder="Select version" />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="latest">Latest</SelectItem>
                              <SelectItem value="stable">Stable</SelectItem>
                              <SelectItem value="canary">Canary</SelectItem>
                              <SelectItem value="dev">Dev</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormDescription>
                            The version of the browser to use
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <Separator />

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <FormField
                      control={form.control}
                      name="operating_system"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Operating System</FormLabel>
                          <Select
                            onValueChange={field.onChange}
                            defaultValue={field.value}
                          >
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue placeholder="Select OS" />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="windows">Windows</SelectItem>
                              <SelectItem value="macos">macOS</SelectItem>
                              <SelectItem value="linux">Linux</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormDescription>
                            The operating system environment
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="work_pool_id"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Work Pool</FormLabel>
                          <Select
                            onValueChange={field.onChange}
                            defaultValue={field.value}
                          >
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue placeholder="Auto-select" />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="">Auto-select</SelectItem>
                              {workPools?.map((pool) => (
                                <SelectItem key={pool.id} value={pool.id}>
                                  {pool.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <FormDescription>
                            Optional: Choose a specific work pool
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Screen Configuration</CardTitle>
                  <CardDescription>
                    Set the viewport dimensions for the browser session
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                    <FormField
                      control={form.control}
                      name="screen.width"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Width (px)</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              {...field}
                              onChange={(e) => field.onChange(Number(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>
                            Screen width in pixels (800-3840)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="screen.height"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Height (px)</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              {...field}
                              onChange={(e) => field.onChange(Number(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>
                            Screen height in pixels (600-2160)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                    <FormField
                      control={form.control}
                      name="screen.dpi"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>DPI</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              {...field}
                              onChange={(e) => field.onChange(Number(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>
                            Screen DPI (72-300)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="screen.scale"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Scale Factor</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              step="0.1"
                              {...field}
                              onChange={(e) => field.onChange(Number(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>
                            Screen scale factor (1-3)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            {/* Advanced Settings */}
            <TabsContent value="advanced" className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle>Advanced Options</CardTitle>
                  <CardDescription>
                    Configure additional browser session settings
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-4">
                    <FormField
                      control={form.control}
                      name="headless"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between">
                          <div className="space-y-0.5">
                            <FormLabel>Headless Mode</FormLabel>
                            <FormDescription>
                              Run browser without a visible UI
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

                    <Separator />

                    <FormField
                      control={form.control}
                      name="record_network"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between">
                          <div className="space-y-0.5">
                            <FormLabel>Record Network Traffic</FormLabel>
                            <FormDescription>
                              Log all network requests and responses
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

                    <Separator />

                    <FormField
                      control={form.control}
                      name="record_console"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between">
                          <div className="space-y-0.5">
                            <FormLabel>Record Console Output</FormLabel>
                            <FormDescription>
                              Log all console messages
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
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            {/* Resource Limits */}
            <TabsContent value="resources" className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle>Resource Limits</CardTitle>
                  <CardDescription>
                    Define resource constraints for the browser session
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                    <FormField
                      control={form.control}
                      name="resource_limits.cpu"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>CPU Cores</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              placeholder="2"
                              {...field}
                              value={field.value || ''}
                              onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                            />
                          </FormControl>
                          <FormDescription>
                            Number of CPU cores (1-8)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="resource_limits.memory"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Memory</FormLabel>
                          <FormControl>
                            <Input
                              placeholder="2048m"
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            Memory limit (e.g., 2048m, 4g)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <FormField
                    control={form.control}
                    name="resource_limits.timeout_minutes"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Timeout (minutes)</FormLabel>
                        <FormControl>
                          <Input
                            type="number"
                            {...field}
                            onChange={(e) => field.onChange(Number(e.target.value))}
                          />
                        </FormControl>
                        <FormDescription>
                          Session timeout in minutes (5-240)
                        </FormDescription>
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
              onClick={() => navigate(-1)}
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
              Create Session
            </Button>
          </div>
        </form>
      </Form>
    </div>
  )
} 