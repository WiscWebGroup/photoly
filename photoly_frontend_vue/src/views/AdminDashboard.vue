<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import { ref, defineComponent, h } from 'vue';
import { useMessage, NIcon } from "naive-ui";
import {
  Settings20Regular as SettingIcon,
  ArrowUpload20Filled as UploadIcon,
  DrinkWine20Regular as WineIcon
} from "@vicons/fluent";
</script>

<template>
  <div>

    <!-- Navigation Bar -->
    <n-space vertical size="large" >
    <n-layout>
      <n-layout-header id="header1">
        <img src="@/assets/logo.ico" id="img1" @click="$router.push('/thepersonincharge/dashboard')"/>
        <n-gradient-text :size="20" :gradient="{
          from: 'rgb(255 255 255)',
          to: 'rgb(245 245 245)'
        }" id="photolyText" @click="$router.push('/thepersonincharge/dashboard')">PHOTOLY ADMIN</n-gradient-text>
        <div id="naviContainer">
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="$router.push('/thepersonincharge/dashboard')">
                Dashboard
            </n-button>
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="$router.push('/thepersonincharge/usermanage')">
                User Mng
            </n-button>
            
        </div>
      </n-layout-header>
    </n-layout>
  </n-space>
  <!-- ENDOF: Navigation Bar -->

  <div style="align-items: center; display: flex; align-content: center; 
    justify-content: center; margin-top: 5vh;">
    <n-space vertical>

      <!-- Server Status Progress Section -->
        <n-card title="" size="medium" style="border-radius: 10px; ">
          <n-space vertical>
            <n-h2>Server Status</n-h2>
            <n-space style="text-align: center;">
              <div>
                <n-h4>Total Status</n-h4>
                <n-progress type="circle" :percentage="percentageServerUp" />
              </div>
              <div>
                <n-h4>MySQL Status</n-h4>
                <n-progress type="circle" :status="mysqlStatus === 100 ? 'success' : 'error'" :percentage="mysqlStatus" />
              </div>
              <div>
                <n-h4>Redis Status</n-h4>
                <n-progress type="circle" :status="redisStatus === 100 ? 'success' : 'error'" :percentage="redisStatus" />
              </div>
              <div>
                <n-h4>Backend Status</n-h4>
                <n-progress type="circle" :status="backendStatus === 100 ? 'success' : 'error'" :percentage="backendStatus" />
              </div>
            </n-space>
          </n-space>
          
        </n-card>
      <!-- ENDOF: Server Status Progress Section -->

      <!-- Addr and Location Section -->
        <n-card title="" size="medium" style="border-radius: 10px; margin-top: 1rem;">
          <n-space vertical>
            <n-h2>Locations</n-h2>
            <n-table :bordered="false" :single-line="false">
              <thead>
                <tr>
                  <th>Icon</th>
                  <th>Name</th>
                  <th>Address</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>
                    <img
                      style="width: 90px; height: 60px; border-radius: 8px;"
                      src="@/assets/MySQL.png"
                    />
                  </td>
                  <td>MySQL</td>
                  <td>{{ addresses.mysql }}</td>
                </tr>

                <tr>
                  <td>
                    <img
                      style="width: 90px; height: 60px; border-radius: 8px;"
                      src="@/assets/Redis.png"
                    />
                  </td>
                  <td>Redis</td>
                  <td>{{ addresses.redis }}</td>
                </tr>

                <tr>
                  <td>
                    <img
                      style="width: 50px; height: 50px; border-radius: 8px;"
                      src="@/assets/Spring.png"
                    />
                  </td>
                  <td>Backend</td>
                  <td>{{ backendAddr }}</td>
                </tr>

                <tr>
                  <td>
                    <n-icon size="50">
                      <UploadIcon />
                    </n-icon>
                  </td>
                  <td>Upload</td>
                  <td>{{ addresses.upload }}</td>
                </tr>

                <tr>
                  <td>
                    <n-icon size="50">
                      <SettingIcon />
                    </n-icon>
                  </td>
                  <td>Configuration</td>
                  <td>{{ addresses.config }}</td>
                </tr>

              </tbody>
            </n-table>

          </n-space>
          
        </n-card>
      <!-- ENDOF: Addr and Location Section -->
      
      <!-- Settings Section -->
        <n-card title="" size="medium" style="border-radius: 10px; margin-top: 1rem; margin-bottom: 2rem;">
          <n-space vertical>
            <n-h2>Settings</n-h2>
            <n-table :bordered="false" :single-line="false">
              <thead>
                <tr>
                  <th>Item</th>
                  <th>Description</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>Registry</td>
                  <td>Allow user to register</td>
                  <td><n-switch v-model:value="userRegistry" @update:value="changeSetting('register')"/></td>
                </tr> 

                <tr>
                  <td>SSafe UUID</td>
                  <td>Enable the S-Safe UUID Mode</td>
                  <td><n-switch v-model:value="SSafeUUIDMode" @update:value="changeSetting('ssafeuuid')"/></td>
                </tr>

                <tr>
                  <td>Token Duration</td>
                  <td>How long will the login token expire</td>
                  <td><n-select v-model:value="tokenDuration" :options="tokenExpireTimeOptions" @update:value="changeSetting('tokenduration')"/></td>
                </tr>

                <tr>
                  <td>Power</td>
                  <td>Power down the backend server</td>
                  <td>
                    <n-button type="error" @click="() => {this.showShutdownModal = true;}" :loading="shutdownBtnLoading">
                      Shut Down
                    </n-button>
                  </td>
                </tr>
              </tbody>
            </n-table>

          </n-space>
        </n-card>
      <!-- ENDOF: Settings Section -->

    </n-space>
    
  </div>
  
  <!-- Modal Section -->

  <!-- This part is for modal of confirming to delete an API -->
  <n-modal v-model:show="showShutdownModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to Shutdown the Backend?

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="powerDown" :loading="shutdownBtnLoading">
            Shutdown
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {this.showShutdownModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
  </n-modal>
  <!-- ENDOF: This part is for modal of confirming to delete an API -->

  <!-- ENDOF: Modal Section -->

  </div>
</template>

<script>

export default defineComponent({
  data() {
    return {
      percentageServerUp: 0,
      mysqlStatus: 0,
      redisStatus: 0,
      backendStatus: 0,

      addresses: {},
      backendAddr: import.meta.env.VITE_APP_BASE_URL,

      userRegistry: false,
      SSafeUUIDMode: false,
      tokenDuration: 0,
      tokenExpireTimeOptions: [
        {
          label: '1 Day',
          value: 1
        },
        {
          label: '3 Days',
          value: 3
        },
        {
          label: '5 Days',
          value: 5
        },
        {
          label: '7 Days',
          value: 7
        },
        {
          label: '14 Days',
          value: 14
        },
        {
          label: '30 Days',
          value: 30
        },
        {
          label: '60 Days',
          value: 60
        },
        {
          label: '90 Days',
          value: 90
        },
        {
          label: '120 Days',
          value: 120
        },
      ],

      showShutdownModal: false,
      shutdownBtnLoading: false,
    }
  },
  setup () {
    return {

    }
  },
  methods: {
    changeSetting (itemToChange) {
      var addrAppendix = "";
      switch (itemToChange) {
        case "register":
          addrAppendix = "setSignUpPermission?permission=" + (this.userRegistry ? 1 : 0);
          break;
        case "ssafeuuid":
          addrAppendix = "setSSafeUUIDMode?mode=" + (this.SSafeUUIDMode ? 1 : 0);
          break;
        case "tokenduration":
          addrAppendix = "setTokenDuration?days=" + this.tokenDuration;
          break;
      }
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/" + addrAppendix,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          window.$message.success("Setting Changed!");
        }else {
          window.$message.error("Error!");
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    powerDown () {
      this.shutdownBtnLoading = true;
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/stop",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        },
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          
        }else {
          window.$message.error("Error!");
        }
      })
      .catch(function (error) { // 请求失败处理
        if (error.code === 'ERR_NETWORK')
        {
          window.$message.success("Backend Stoped!");
          this.shutdownBtnLoading = false;
          this.showShutdownModal = false;
          this.backendStatus = 0;
          this.totalPercentageCal();
        }
        window.$message.error(error);
      });
    },
    querySettings () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/getSettings",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var msgT = response.data.t;
          this.userRegistry = msgT.SignUp === '1';
          this.SSafeUUIDMode = msgT.SSafeUUID === '1';
          this.tokenDuration = msgT.TokenDuration;
        }else {
          window.$message.error("Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    totalPercentageCal () {
      this.percentageServerUp = (this.mysqlStatus + this.redisStatus + this.backendStatus) / 3;
    },
    pingMySQL () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/pingMySQL",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.mysqlStatus = 100;
          this.totalPercentageCal();
        }else {
          window.$message.error("You are not an admin!")
          $router.push('/login');
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
        $router.push('/login');
      });
    },
    pingRedis () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/pingRedis",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.redisStatus = 100;
          this.totalPercentageCal();
        }else {
          window.$message.error("You are not an admin!")
          $router.push('/login');
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
        $router.push('/login');
      });
    },
    getAddresses () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/getAddress",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.addresses = response.data.t;
        }else {
          window.$message.error("You are not an admin!")
          $router.push('/login');
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
        $router.push('/login');
      });
    },
    checkIfIsAdmin () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/isAdmin",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.t)
        {
          this.backendStatus = 100;
          this.totalPercentageCal();
        }else {
          window.$message.error("You are not an admin!")
          $router.push('/login');
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
        $router.push('/login');
      });
    },
  },
  computed: {

  },
  mounted () {
    window.$message = useMessage();
    this.checkIfIsAdmin();
    this.pingMySQL();
    this.pingRedis();
    this.getAddresses();
    this.querySettings();
  },
  components: {

  },
  props: {
    
  }
})

</script>

<style scoped>
#header1 {
  font-size: 20px;
  background-color: #38b2ac;
  height: 4rem;
}
#img1 {
  width: 50px;
  border-radius: 30px;
  vertical-align: middle;
  padding-left: 0.4rem;
  padding-top: 0.4rem;
  padding-bottom: 0.4rem;
  margin-left: 10px;
  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}
#photolyText {
  vertical-align: middle;
  margin-left: 10px;
  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}
#naviContainer {
  float:right;
  padding-top: 0.5rem;
  margin-right: 20px;
  
}
.naviBarBtn {
  color: aliceblue;
  font-size: 1rem;
  margin-top: 0.4rem;
}
.naviBarBtn:hover {
  color: #EDF2F7;
}
#userImg {
  width: 45px;
  border-radius: 30px;
  vertical-align:top;


  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
  
}
.n-menu .n-menu-item-content .n-menu-item-content-header {
  color: white;
}
</style>
