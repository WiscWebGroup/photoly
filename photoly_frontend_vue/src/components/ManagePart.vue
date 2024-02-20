<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import { ref, defineComponent, nextTick, h } from 'vue';
import { NTag } from "naive-ui";
import { useMessage, NIcon } from 'naive-ui'
import { isYesterday, addDays } from "date-fns/esm";

import { ArrowUpload20Filled, Copy20Regular, DocumentEdit20Regular, Delete20Regular, AddCircle20Regular } from "@vicons/fluent"
</script>

<template>
  <div style="align-items: center; display: flex; align-content: center; 
    justify-content: center; margin-top: 5vh;" >
    <n-flex vertical size='large'>
    <n-space v-show="userInfo.userId === -1">
      <n-spin size="large" />
    </n-space>

      <!-- This part is for show the user's basic information -->
      <n-card title="" size="huge" style="border-radius: 10px; background-image: linear-gradient(to right, rgb(178, 245, 234), rgb(190, 227, 248)); "
       v-show="userInfo.userId !== -1">
        <n-flex horizontal size='large'>
          <div>
            <n-badge :value="userInfo.role" >
              <n-avatar :src="avatarAddr" :size="150" round/>
            </n-badge>
          </div>
          
          <div style="margin-left: 1rem;">
            <n-h2>{{ userInfo.userName }}</n-h2>
            <n-p>email: {{ userInfo.email }}</n-p>
            <n-p>created:  {{ userInfo.createDate.split("T")[0] }}</n-p>
          </div>
        </n-flex>

        <template #action>
          <n-space style="" horizontal>
            <n-button strong secondary type="default" style="" @click="loadEditInfo">Edit Info</n-button>
            <n-divider vertical/>
            <n-button strong secondary type="info" style="" @click="loadEditAvatar">Edit Avatar</n-button>
          </n-space>
        </template>
      </n-card>
      
      <!-- ENDOF: This part is for show the user's basic information -->

      <!-- This part is for showing the carousel -->
      <n-carousel autoplay show-arrow style="border-radius: 10px; width: 50vw; margin-top: 1rem;">
      <img
        class="carousel-img"
        src="@/assets/JH_20211030_5406.jpg"
      >
      <img
        class="carousel-img"
        src="@/assets/JH_20220601_5686.jpg"
      >
      <img
        class="carousel-img"
        src="@/assets/JH_20220710_5856.jpg"
      >
    </n-carousel>
    <!-- ENDOF: This part is for showing the carousel -->

    <n-calendar
      v-model:value="calendarValue"
      #="{ year, month, date }"
      @update:value="handleUpdateCalendarValue"
      style="margin-top: 1rem;"
    >
      {{ year }}-{{ month }}-{{ date }}
    </n-calendar>

    <n-card title="" size="medium" style="border-radius: 10px; margin-bottom: 5vh;"
       v-show="userInfo.userId !== -1">
       <n-space vertical>
          <n-h2>
            API Table
            <n-button strong secondary circle type="success" @click="() => {this.showAddAPIModal = true;}">
              <template #icon>
                <n-icon><AddCircle20Regular /></n-icon>
              </template>
            </n-button>
        </n-h2>
          <n-empty v-show="myAPIList.length <= 0" style="padding-left: 0; padding-top: 0;">
          </n-empty>
          <n-table :bordered="false" :single-line="false" v-show="myAPIList.length > 0">
            <thead>
              <tr>
                <th>ID</th>
                <th>Authorization</th>
                <th>Token</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody v-for="api in myAPIList">
              <tr>
                <td>{{ api.credId }}</td>
                <td>{{ api.authorization }}</td>
                <td>{{ api.token }}</td>
                <td>
                  <n-space>
                    <n-button strong secondary circle type="default" @click="copyToClipboard(api.token)">
                      <template #icon>
                        <n-icon><Copy20Regular /></n-icon>
                      </template>
                    </n-button>
                    <n-button strong secondary circle type="info" @click="prepareAPIEdit(api.credId, api.authorization)">
                      <template #icon>
                        <n-icon><DocumentEdit20Regular /></n-icon>
                      </template>
                    </n-button>
                    <n-button strong secondary circle type="error" @click="prepareAPIDelete(api.credId)">
                      <template #icon>
                        <n-icon><Delete20Regular /></n-icon>
                      </template>
                    </n-button>
                  </n-space>
                </td>
              </tr>
            </tbody>
          </n-table>

          <n-h2 style="margin-top: 1rem;">API Documents</n-h2>
          <n-collapse>
            <n-collapse-item title="API functionalities" name="1">
              <n-blockquote>
                <n-p>
                  C: Able to upload photo (Create)
                </n-p>
                <n-p>
                  R: Able to query namespace, photos in namespaces and render the photo (Read)
                </n-p>
                <n-p>
                  D: Able to delete photo (Delete)
                </n-p>
              </n-blockquote>
              
            </n-collapse-item>
            <n-collapse-item title="How to use API in external build" name="2">
              <n-h4>C1 Upload Photo - POST:</n-h4>
              <n-blockquote>
                <n-p>
                  Addr: server_path/cred/uploadPhoto/YOUR_TOKEN
                </n-p>
                <n-p>
                  Takein: file:MultipartFile | photo:JSONText containing nsId(num) and visibility(num)
                </n-p>
                <n-p>
                  Return: a string that is the photo's uuid
                </n-p>
              </n-blockquote>

              <n-h4>C2 Upload Photos - POST:</n-h4>
              <n-blockquote>
                <n-p>
                  Addr: server_path/cred/uploadPhotos/YOUR_TOKEN
                </n-p>
                <n-p>
                  Takein: files:MultipartFile[] | photos:JSONText containing list of photos( nsId(num) and visibility(num) )
                </n-p>
                <n-p>
                  Return: a list of string of photo uuids
                </n-p>
              </n-blockquote>

              <n-h4>D Delete Photo - POST:</n-h4>
              <n-blockquote>
                <n-p>
                  Addr: server_path/cred/deletePhoto/YOUR_TOKEN
                </n-p>
                <n-p>
                  Takein: uuid: str, photo's uuid
                </n-p>
                <n-p>
                  Return: an int, 1 if successful
                </n-p>
              </n-blockquote>

              <n-h4>R1 Render Photo - POST:</n-h4>
              <n-blockquote>
                <n-p>
                  Addr: server_path/cred/render/YOUR_TOKEN
                </n-p>
                <n-p>
                  Takein: uuid: str, photo's uuid
                </n-p>
                <n-p>
                  Return: a byte[] that is the image (could be seen directly in browser url)
                </n-p>
              </n-blockquote>

              <n-h4>R2 Get RootNS - GET:</n-h4>
              <n-blockquote>
                <n-p>
                  Addr: server_path/cred/queryRootNamespace/YOUR_TOKEN
                </n-p>
                <n-p>
                  Takein: None
                </n-p>
                <n-p>
                  Return: an JSON object that is the Root namespace
                </n-p>
              </n-blockquote>

              <n-h4>R3 Get NS - GET:</n-h4>
              <n-blockquote>
                <n-p>
                  Addr: server_path/cred/queryNamespaces/YOUR_TOKEN
                </n-p>
                <n-p>
                  Takein: parentId: int, the parentId of ns
                </n-p>
                <n-p>
                  Return: an JSON object that is the list of namespaces under parentId
                </n-p>
              </n-blockquote>

              <n-h4>R4 Get Photo List - GET:</n-h4>
              <n-blockquote>
                <n-p>
                  Addr: server_path/cred/queryPhotoList/YOUR_TOKEN
                </n-p>
                <n-p>
                  Takein: nsId: int, the namespace Id
                </n-p>
                <n-p>
                  Return: an JSON object that is the list of photos' meta info
                </n-p>
              </n-blockquote>
            </n-collapse-item>
          </n-collapse>
       </n-space>
    
    </n-card>

    </n-flex>

    <!-- MODAL PART -->

    <!-- This part is for modal of edit user info -->
    <n-modal v-model:show="showEditUserInfoModal">
      <n-card title="Edit Information" size="huge" style="width: 400px; border-radius: 10px;" role="dialog"
        aria-modal="true" :bordered="false">
            <n-space vertical>

                Username: <n-input v-model:value="editUserInfo.userName" type="text" placeholder="User Name" />
                Email: <n-input v-model:value="editUserInfo.email" type="text" placeholder="Email" />

                <n-divider />

                Current Password: <n-input v-model:value="oldPassword" type="password" placeholder="Confirm Password" />
                New Password: <n-input v-model:value="editUserInfo.password" type="password" placeholder="Password" />
                Confirm Password: <n-input v-model:value="confirmPassword" type="password" placeholder="Confirm Password" />

                <n-space style="margin-top: 1rem">
                  <n-button type="success" round size="large" style="margin-top: 1rem; width: 100%;" @click="doEdit">
                    Submit
                  </n-button>
                  <n-button round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {this.showEditUserInfoModal = false;}">
                    Cancel
                  </n-button>
                </n-space>
            </n-space>
      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of edit user info -->

    <!-- This part is for modal of edit user avatar -->
    <n-modal v-model:show="showEditUserAvatarModal">
      <n-card title="Edit Information" size="huge" style="width: 400px; border-radius: 10px;" role="dialog"
        aria-modal="true" :bordered="false">
            <n-space vertical>

              <n-upload
                  :custom-request="addPhotoToArea"
                >
                  <n-button strong secondary type="info" style="">
                    <template #icon>
                      <n-icon>
                        <ArrowUpload20Filled />
                      </n-icon>
                    </template>
                      Upload Avatar
                  </n-button>
              </n-upload>

                <n-space style="margin-top: 1rem">
                  <n-button type="success" round size="large" style="margin-top: 1rem; width: 100%;" @click="doAvatarUpdate">
                    Submit
                  </n-button>
                  <n-button round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {this.showEditUserAvatarModal = false;}">
                    Cancel
                  </n-button>
                </n-space>
            </n-space>
      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of edit user avatar -->

    <!-- This part is for modal of edit API -->
    <n-modal v-model:show="showEditAPIModal">
      <n-card title="Edit API" size="huge" style="width: 400px; border-radius: 10px;" role="dialog"
        aria-modal="true" :bordered="false">
            <n-space vertical>
                <n-space>
                  ID: {{ APIEditID }}
                </n-space>
                <n-space>
                  <n-p>Authorization: </n-p>
                  <n-checkbox-group v-model:value="APIEditAuthorization">
                    <n-space item-style="display: flex;">
                      <n-checkbox value="C" label="Upload" />
                      <n-checkbox value="R" label="Read" />
                      <n-checkbox value="D" label="Delete" />
                    </n-space>
                </n-checkbox-group>
                </n-space>
                
                

                <n-space style="margin-top: 1rem">
                  <n-button type="success" round size="large" style="margin-top: 1rem; width: 100%;" @click="doAPIUpdate">
                    Update
                  </n-button>
                  <n-button round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {this.showEditAPIModal = false;}">
                    Cancel
                  </n-button>
                </n-space>
            </n-space>
      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of edit API -->

     <!-- This part is for modal of confirming to delete an API -->
     <n-modal v-model:show="showDeleteAPIModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to DELETE this API?
        <p>*The process cannot be reversed</p>

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="doAPIDelete">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {this.showDeleteAPIModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to delete an API -->

    <!-- This part is for modal of Add API -->
    <n-modal v-model:show="showAddAPIModal">
      <n-card title="Add API" size="huge" style="width: 400px; border-radius: 10px;" role="dialog"
        aria-modal="true" :bordered="false">
            <n-space vertical>
              <n-p>Token: </n-p>
                  <n-checkbox-group v-model:value="APIAddAuthorization">
                    <n-space item-style="display: flex;">
                      <n-checkbox value="C" label="Upload" />
                      <n-checkbox value="R" label="Read" />
                      <n-checkbox value="D" label="Delete" />
                    </n-space>
                </n-checkbox-group>              

                <n-space style="margin-top: 1rem">
                  <n-button type="success" round size="large" style="margin-top: 1rem; width: 100%;" @click="doAPIAdd">
                    Create
                  </n-button>
                  <n-button round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {this.showAddAPIModal = false;}">
                    Cancel
                  </n-button>
                </n-space>
            </n-space>
      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of Add API -->

    <!-- ENDOF: MODAL PART -->
    
  
    <n-back-top :right="100" />
  </div>
</template>

<script>

export default defineComponent({
  data() {
    return {
      userInfo: {
        "userId": -1,
        "userName": "",
        "email": "",
        "password": "",
        "createDate": "1999-01-01T",
        "role": "user"
      },
      calendarValue: ref(addDays(Date.now(), 0).valueOf()),
      showEditUserInfoModal: false,
      editUserInfo: {
        "userId": -1,
        "userName": "",
        "email": "",
        "password": "",
      },
      oldPassword: "",
      confirmPassword: "",

      showEditUserAvatarModal: false,
      uploadPhotoFiles: new FormData(),
      fileList: [],

      avatarAddr: import.meta.env.VITE_APP_BASE_URL + "/user/getAvatar/" + localStorage.getItem("HRD-Token") + "?random=" + Math.floor(Math.random() * 1000),

      myAPIList: [],
      
      showEditAPIModal: false,
      APIEditID: -1,
      APIEditAuthorization: [],

      showDeleteAPIModal: false,
      APIDeleteID: -1,

      showAddAPIModal: false,
      APIAddAuthorization: [],
    }
  },
  setup () {
    return {

    }
  },
  methods: {
    doAPIAdd () {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/cred/create",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
          "authorization": this.APIAddAuthorization.join('')
        }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          window.$message.success("API Created!");
          this.showAddAPIModal = false;
          this.getMyAPIs();

          this.APIAddAuthorization = [];
        }else {
          window.$message.error("Create API Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    doAPIDelete () {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/cred/delete?credId=" + this.APIDeleteID,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          window.$message.success("Delete Success!");
          this.showDeleteAPIModal = false;
          this.getMyAPIs();

          this.APIDeleteID = -1;
        }else {
          window.$message.error("Update API Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    prepareAPIDelete (id) {
      this.showDeleteAPIModal = true;
      this.APIDeleteID = id;
    },
    doAPIUpdate () {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/cred/update",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
          "authorization": this.APIEditAuthorization.join(''),
          "credId": this.APIEditID
        }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          window.$message.success("Update Success!");
          this.showEditAPIModal = false;
          this.getMyAPIs();

          this.APIEditID = -1;
          this.APIEditAuthorization = [];

        }else {
          window.$message.error("Update API Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
      
    },
    prepareAPIEdit (id, auth) {
      this.showEditAPIModal = true;
      this.APIEditAuthorization = auth.split('');
      this.APIEditID = id;
    },
    copyToClipboard (token) {
      navigator.clipboard.writeText(token);
      window.$message.success("Copied to the Clipboard!")
    },
    getMyAPIs () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/cred/query",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {}
      }).then((response) => {
        if (response.data.msgCode === 200)
        {

          this.myAPIList = response.data.t;
        }else {
          window.$message.error("Get API Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    doAvatarUpdate () {
      if (!this.uploadPhotoFiles.has("file"))
      {
        window.$message.warning("No Avatar Selected!");
      }else {
        axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/user/updateAvatar",
            headers: {
              'Content-Type': 'multipart/form-data',
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: this.uploadPhotoFiles
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              window.$message.success("Avatar updated!");
              this.getUserInfo();
              this.uploadPhotoFiles = new FormData();
              this.fileList = [];
              this.avatarAddr = import.meta.env.VITE_APP_BASE_URL + "/user/getAvatar/" + localStorage.getItem("HRD-Token") + "?random=" + Math.floor(Math.random() * 1000);
              this.showEditUserAvatarModal = false;
            }else {
              window.$message.error("Avatar update Response Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
      }
    },
    addPhotoToArea({file}) {
      this.uploadPhotoFiles.append("file", file.file);
    },
    loadEditAvatar() {
      this.showEditUserAvatarModal = true;
      this.uploadPhotoFiles = new FormData()
      this.fileList = []
    },
    doEdit () {
      if (this.editUserInfo.userName === "" && this.editUserInfo.email === "" && this.editUserInfo.password === "")
      {
        window.$message.warning("Nothing to update about!")
      }else if (this.editUserInfo.password !== this.confirmPassword){
        window.$message.warning("Passwords are not equal!")
      }else if (!this.checkInput()) {
        window.$message.warning("Wrong Email Format!")
      }else{
        if (this.editUserInfo.userName !== "" && this.editUserInfo.userName !== null && this.editUserInfo.userName !== this.userInfo.userName)
        {
          axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/user/updateUsername?username=" + this.editUserInfo.userName,
            headers: {
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
          }
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              window.$message.success("Username updated!");
              this.getUserInfo();
            }else {
              window.$message.error("Username update Response Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
        }
        if (this.editUserInfo.email !== "" && this.editUserInfo.email !== null && this.editUserInfo.email !== this.userInfo.email)
        {
          axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/user/updateEmail?email=" + this.editUserInfo.email,
            headers: {
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
          }
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              window.$message.success("Email updated!");
              this.getUserInfo();
            }else {
              window.$message.error("Email update Response Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
        }
        if (this.editUserInfo.password !== "" && this.editUserInfo.password !== null && this.editUserInfo.password !== this.userInfo.password)
        {
          axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/user/updatePassword?oldPass=" + this.oldPassword +"&newPass=" + this.editUserInfo.email,
            headers: {
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
          }
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              window.$message.success("Password updated!");
              this.getUserInfo();
            }else {
              window.$message.error("Password update Response Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
        }
        
        this.showEditUserInfoModal = false;
      }
    },
    checkInput() {
      var re = new RegExp(".+@.+\..+");
      if (this.editUserInfo.email !== "" && re.test(this.editUserInfo.email))
      {
        return true
      }
      return false
    },
    loadEditInfo() {
      this.showEditUserInfoModal = true;
      this.editUserInfo.userId = this.userInfo.userId;
      this.editUserInfo.userName = this.userInfo.userName;
      this.editUserInfo.email = this.userInfo.email;
    },
    getUserInfo () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/user/getInfo/",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          let userT = response.data.t
          this.userInfo = userT;
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    handleUpdateCalendarValue(_, { year, month, date }) {
        // message.success(`${year}-${month}-${date}`);
    },
  },
  computed: {

  },
  mounted () {
    window.$message = useMessage();
    this.getUserInfo();
    this.getMyAPIs();
  },
  components: {

  },
  props: {
    
  }
})

</script>

<style scoped>
.carousel-img {
  width: 100%;
  height: 240px;
  object-fit: cover;
}
</style>
