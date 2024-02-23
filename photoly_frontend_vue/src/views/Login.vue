<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";

</script>

<template>
  
  <div class="centralContainer">
    
    <div class="register">
      <n-card title="Login">
        <n-alert title="Warning" type="error" style="border-radius: 15px; margin-bottom: 1rem" v-show="warningShow">
          {{ errors }}
        </n-alert>
        <n-space vertical size="large">
        
          <n-auto-complete
            v-model:value="emailInput"
            placeholder="Email"
            clearable
            @keydown.enter="login"
          />
          <n-input
            type="password"
            show-password-on="mousedown"
            placeholder="Password"
            v-model:value="password"
            @keydown.enter="login"
            id="passwordTextbox"
          />

          <n-button strong secondary round style="width: 100%;" type="primary" :loading="loading" @click="login" >
            Login
          </n-button>

          <n-button strong secondary round style="width: 100%;" @click="toSignup" >
            Signup
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
      emailInput: "",
      password: "",
      loading: false,
      warningShow: false,
      errors: "",
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
    checkInput() {
      var re = new RegExp(".+@.+\..+");
      if (this.emailInput !== "" && re.test(this.emailInput) && this.password !== "")
      {
        return true
      }
      return false
    },
    showWarning(error) {
      this.warningShow = true
      this.errors = error
      setTimeout(() => {this.warningShow = false}, 5000);
    },
    async login() {
      this.loading = true;
      if (this.checkInput())
      {
        await axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/user/signIn/",
          headers: {},
          data: {
            "email": this.emailInput,
            "password": this.password,
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            // success
            localStorage.setItem("HRD-Token", response.data.token);
            router.push('/home/HomePart');
          }else {
            this.showWarning("Wrong Email or Password!");
            this.loading = false;
          }

        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          this.showWarning(error);
          this.loading = false;
        });
      }else {
        this.loading = false;
        this.showWarning("Please input correct email and password format!")
      }
      
    }
},
  computed: {

  },
  mounted () {
    if (localStorage.getItem("HRD-Token") !== null)
    {
      axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/user/pingToken/",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            router.push('/home/HomePart');
          }
        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          this.showWarning(error);
        });
    }
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
