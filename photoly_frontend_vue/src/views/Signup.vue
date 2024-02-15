<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
</script>

<template>
  <div class="centralContainer">
    <div class="register">
      <n-card title="Login">
        
        <n-space vertical size="large">
          <n-auto-complete
            v-model:value="username"
            placeholder="Username"
            clearable
          />
          <n-auto-complete
            v-model:value="emailInput"
            placeholder="Email"
            clearable
          />

          <n-auto-complete
            v-model:value="password"
            placeholder="Password"
            clearable
            id="passwordTextbox"
            
          />

          <n-button strong secondary round style="width: 100%;" type="primary" :loading="loading" @click="signup">
            Signup
          </n-button>

          <n-button strong secondary round style="width: 100%;" @click="toLogin">
            Already have an account?
          </n-button>
        </n-space>      
      
      </n-card>
      
    </div>
    
  </div>
</template>

<script>

export default{
  data() {
    return {
      username: "",
      emailInput: "",
      password: "",
      loading: false,
    }
  },
  setup () {
    return {
      
    }
  },
  methods: {
    toLogin: function() {
      router.push('/login');
    },
    toSignup: function () {
      router.push('/signup');
    },
    signup() {
      this.loading = true;
    }
  },
  computed: {

  },
  mounted () {
    
  },
  components: {

  }
}

</script>

<style scoped>
  .centralContainer {
    padding: 0;
    margin: 0;
    display: block;
    height: 100vh;
    background-size: cover;
    background-attachment: fixed;
    background-position: center 0;
    background-repeat: no-repeat;
    background-image: url("@/assets/JH_20211030_5406.jpg");
  }
  .register {
    display:flex;
    align-items: center;
    -webkit-box-pack: center;
    justify-content: center;
    padding: 2rem;
  }
  .n-card {
    max-width: 30vw;
    position: fixed;
    top: 50%;
    left: 50%;
    -webkit-transform: translateX(-50%) translateY(-70%);
    background-color: rgba(255, 255, 255, 0.92);
  }
</style>
