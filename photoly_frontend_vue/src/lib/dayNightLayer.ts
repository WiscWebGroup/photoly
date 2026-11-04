import { createTileMesh } from 'maplibre-gl'
import type { CustomLayerInterface, CustomRenderMethodInput, Map as GLMap } from 'maplibre-gl'
import {
  globeLightingStrength,
  solarPosition,
  surfaceNormal,
  type Direction,
} from './solarPosition'

type Mesh = {
  x: number
  y: number
  vao: WebGLVertexArrayObject
  buffers: WebGLBuffer[]
  count: number
}
type Program = { program: WebGLProgram; uniforms: Record<string, WebGLUniformLocation | null> }

/** A geographic overlay, drawn below photo heat/markers. No per-frame clock or network work. */
export class DayNightLayer implements CustomLayerInterface {
  readonly id = 'globe-day-night'
  readonly type = 'custom' as const
  readonly renderingMode = '2d' as const
  private map?: GLMap
  private meshes: Mesh[] = []
  private programs = new Map<string, Program>()
  private sun: Direction = solarPosition(new Date()).direction
  private failed = false

  constructor(private readonly onFailure: () => void) {}

  update(date: Date) {
    this.sun = solarPosition(date).direction
    this.map?.triggerRepaint()
  }

  onAdd(map: GLMap) {
    this.map = map
  }

  private shader(gl: WebGL2RenderingContext, type: number, source: string) {
    const shader = gl.createShader(type)
    if (!shader) throw new Error('Unable to allocate illumination shader')
    gl.shaderSource(shader, source)
    gl.compileShader(shader)
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
      const message = gl.getShaderInfoLog(shader)
      gl.deleteShader(shader)
      throw new Error(message || 'Unable to compile illumination shader')
    }
    return shader
  }

  private program(gl: WebGL2RenderingContext, data: CustomRenderMethodInput['shaderData']) {
    const cached = this.programs.get(data.variantName)
    if (cached) return cached
    const shaders: WebGLShader[] = []
    const program = gl.createProgram()
    if (!program) throw new Error('Unable to allocate illumination program')
    try {
      shaders.push(
        this.shader(
          gl,
          gl.VERTEX_SHADER,
          `#version 300 es
        precision highp float;
        ${data.vertexShaderPrelude}
        ${data.define}
        layout(location = 0) in vec2 a_pos;
        layout(location = 1) in vec3 a_normal;
        out vec3 v_normal;
        void main() {
          gl_Position = projectTile(a_pos, a_pos);
          v_normal = a_normal;
        }`,
        ),
      )
      shaders.push(
        this.shader(
          gl,
          gl.FRAGMENT_SHADER,
          `#version 300 es
        precision highp float;
        in vec3 v_normal;
        uniform vec3 u_sun;
        uniform float u_strength;
        out vec4 fragColor;
        void main() {
          float altitude = asin(clamp(dot(normalize(v_normal), u_sun), -1.0, 1.0));
          float night = 1.0 - smoothstep(radians(-12.0), radians(3.0), altitude);
          float alpha = 0.52 * night * u_strength;
          fragColor = vec4(vec3(7.0, 20.0, 38.0) / 255.0 * alpha, alpha);
        }`,
        ),
      )
      shaders.forEach((shader) => gl.attachShader(program, shader))
      gl.linkProgram(program)
      if (!gl.getProgramParameter(program, gl.LINK_STATUS))
        throw new Error(gl.getProgramInfoLog(program) || 'Unable to link illumination program')
      const uniforms = Object.fromEntries(
        [
          'u_projection_matrix',
          'u_projection_fallback_matrix',
          'u_projection_tile_mercator_coords',
          'u_projection_clipping_plane',
          'u_projection_transition',
          'u_sun',
          'u_strength',
        ].map((name) => [name, gl.getUniformLocation(program, name)]),
      )
      const result = { program, uniforms }
      this.programs.set(data.variantName, result)
      return result
    } catch (error) {
      gl.deleteProgram(program)
      throw error
    } finally {
      shaders.forEach((shader) => gl.deleteShader(shader))
    }
  }

  private prepareMeshes(gl: WebGL2RenderingContext) {
    // Four z1 tiles avoid a wrapped z0 seam. Pole vertices use the public mesh helper.
    for (let y = 0; y < 2; y++)
      for (let x = 0; x < 2; x++) {
        const mesh = createTileMesh(
          {
            granularity: 64,
            generateBorders: false,
            extendToNorthPole: y === 0,
            extendToSouthPole: y === 1,
          },
          '16bit',
        )
        const vao = gl.createVertexArray()
        if (!vao) throw new Error('Unable to allocate illumination mesh')
        const entry: Mesh = { x, y, vao, buffers: [], count: mesh.indices.byteLength / 2 }
        this.meshes.push(entry)
        gl.bindVertexArray(vao)
        const upload = (target: number, data: ArrayBuffer | ArrayBufferView) => {
          const buffer = gl.createBuffer()
          if (!buffer) throw new Error('Unable to allocate illumination buffer')
          entry.buffers.push(buffer)
          gl.bindBuffer(target, buffer)
          gl.bufferData(target, data, gl.STATIC_DRAW)
        }
        upload(gl.ARRAY_BUFFER, mesh.vertices)
        gl.enableVertexAttribArray(0)
        gl.vertexAttribPointer(0, 2, gl.SHORT, false, 0, 0)
        const positions = new Int16Array(mesh.vertices)
        const normals = new Float32Array((positions.length / 2) * 3)
        for (let i = 0; i < positions.length; i += 2) {
          const px = positions[i]!,
            py = positions[i + 1]!
          const longitude = ((x + px / 8192) / 2) * 360 - 180
          const latitude =
            py === -32768
              ? 90
              : py === 32767
                ? -90
                : (Math.atan(Math.sinh(Math.PI * (1 - (y + py / 8192)))) * 180) / Math.PI
          normals.set(surfaceNormal(longitude, latitude), (i / 2) * 3)
        }
        upload(gl.ARRAY_BUFFER, normals)
        gl.enableVertexAttribArray(1)
        gl.vertexAttribPointer(1, 3, gl.FLOAT, false, 0, 0)
        upload(gl.ELEMENT_ARRAY_BUFFER, mesh.indices)
      }
    gl.bindVertexArray(null)
  }

  render(gl: WebGL2RenderingContext, args: CustomRenderMethodInput) {
    if (this.failed || !this.map) return
    const strength = globeLightingStrength(this.map.getZoom())
    if (!strength) return
    try {
      const { program, uniforms: u } = this.program(gl, args.shaderData)
      if (!this.meshes.length) this.prepareMeshes(gl)
      gl.useProgram(program)
      gl.disable(gl.DEPTH_TEST)
      gl.disable(gl.STENCIL_TEST)
      gl.disable(gl.CULL_FACE)
      gl.enable(gl.BLEND)
      gl.blendEquation(gl.FUNC_ADD)
      gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
      gl.uniform3fv(u.u_sun!, this.sun)
      gl.uniform1f(u.u_strength!, strength)
      for (const mesh of this.meshes) {
        const data = args.getProjectionData({
          tileID: { canonical: { x: mesh.x, y: mesh.y, z: 1 }, wrap: 0 },
          applyGlobeMatrix: true,
        })
        gl.uniformMatrix4fv(u.u_projection_matrix!, false, data.mainMatrix)
        gl.uniformMatrix4fv(u.u_projection_fallback_matrix!, false, data.fallbackMatrix)
        gl.uniform4fv(u.u_projection_tile_mercator_coords!, data.tileMercatorCoords)
        gl.uniform4fv(u.u_projection_clipping_plane!, data.clippingPlane)
        gl.uniform1f(u.u_projection_transition!, data.projectionTransition)
        gl.bindVertexArray(mesh.vao)
        gl.drawElements(gl.TRIANGLES, mesh.count, gl.UNSIGNED_SHORT, 0)
      }
    } catch {
      this.failed = true
      // Mutate the style only after the current render pass has finished.
      queueMicrotask(this.onFailure)
    } finally {
      gl.bindVertexArray(null)
    }
  }

  onRemove(_map: GLMap, gl: WebGL2RenderingContext) {
    this.meshes.forEach((mesh) => {
      gl.deleteVertexArray(mesh.vao)
      mesh.buffers.forEach((buffer) => gl.deleteBuffer(buffer))
    })
    this.programs.forEach(({ program }) => gl.deleteProgram(program))
    this.meshes = []
    this.programs.clear()
    this.map = undefined
  }
}
